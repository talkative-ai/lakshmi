package routes

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/talkative-ai/core/common"
	"github.com/talkative-ai/core/db"
	"github.com/talkative-ai/core/models"
	"github.com/talkative-ai/core/myerrors"
	"github.com/talkative-ai/core/prehandle"
	"github.com/talkative-ai/core/redis"
	"github.com/talkative-ai/core/router"
	uuid "github.com/talkative-ai/go.uuid"
	"github.com/talkative-ai/lakshmi/compile"
	"github.com/talkative-ai/lakshmi/helpers"
)

// PostPublish router.Route
// Path: "/user/register",
// Method: "GET",
// Accepts models.TokenValidate
// Responds with status of success or failure
var PostPublish = &router.Route{
	Path:       "/v1/publish/{id:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}/{version:[0-9]*}",
	Method:     "POST",
	Handler:    http.HandlerFunc(PostPublishHandler),
	Prehandler: []prehandle.Prehandler{prehandle.SetJSON},
}

func PostPublishHandler(w http.ResponseWriter, r *http.Request) {

	// TODO: Deny certain publish statuses
	urlparams := mux.Vars(r)
	projectID, err := uuid.FromString(urlparams["id"])
	if err != nil {
		myerrors.Respond(w, &myerrors.MySimpleError{
			Code:    http.StatusBadRequest,
			Log:     err.Error(),
			Req:     r,
			Message: "Invalid project-id",
		})
		return
	}

	var version int64
	publishID := projectID.String()

	demo := r.URL.Query().Get("demo")
	var isDemo bool
	if demo != "" {
		isDemo = true
	}

	if isDemo {
		version = -1
		publishID = fmt.Sprintf("demo:%+v", publishID)
		helpers.CreateVersionedProject(nil, projectID.String(), -1)
	} else {
		var err error
		version, err = strconv.ParseInt(urlparams["version"], 10, 64)
		if err != nil {
			myerrors.Respond(w, &myerrors.MySimpleError{
				Code:    http.StatusBadRequest,
				Log:     err.Error(),
				Req:     r,
				Message: "Invalid version number",
			})
			return
		}

	}

	err = initiateCompiler(projectID, publishID, version, isDemo)
	if err != nil {
		common.RedisSET(
			fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(publishID), "status"),
			[]byte(fmt.Sprintf("%v", models.PublishStatusProblem))).Exec(redis.Instance)
		myerrors.Respond(w, &myerrors.MySimpleError{
			Code: http.StatusInternalServerError,
			Log:  err.Error(),
			Req:  r,
		})
		return
	}
}

type SyncGroup struct {
	wg     sync.WaitGroup
	wgMu   sync.Mutex
	wgSema uint8
}

func initiateCompiler(projectID uuid.UUID, publishID string, version int64, isDemo bool) error {

	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(publishID), "status"),
		[]byte(fmt.Sprintf("%v", models.PublishStatusPublishing)))

	var project models.VersionedProject
	err := db.DBMap.SelectOne(&project, `
			SELECT *
			FROM static_published_projects_versioned
			WHERE "ProjectID"=$1
			AND "Version"=$2
		`, projectID, version)
	if err != nil {
		return err
	}

	projectItems := project.ProjectData
	for idx := range projectItems {
		projectItems[idx].ProjectID = projectID
	}

	triggerItems := project.TriggerData
	for idx := range triggerItems {
		triggerItems[idx].ProjectID = projectID
	}

	// Delete old published data
	membersSlice := redis.Instance.SMembers(fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(publishID), "keys"))
	redis.Instance.Del(membersSlice.Val()...)

	redisWriter := make(chan common.RedisCommand, 10)
	defer close(redisWriter)

	swg := SyncGroup{}

	trackRedisKeys := true
	ignoreTrack := map[string]bool{
		models.KeynavGlobalMetaProjects():             true,
		models.KeynavProjectMetadataStatic(publishID): true,
	}

	go func() {
		swg.wgMu.Lock()
		swg.wg.Add(1)
		swg.wgMu.Unlock()
		for command := range redisWriter {
			if command.Key == "KILL" {
				break
			}

			swg.wgMu.Lock()
			swg.wg.Add(1)
			command.Exec(redis.Instance)

			// Track all saved keys so that later we can remove them all in a republish
			if trackRedisKeys && !ignoreTrack[command.Key] {
				common.RedisSADD(fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(publishID), "keys"), []byte(command.Key)).Exec(redis.Instance)
			}
			swg.wg.Done()
			swg.wgMu.Unlock()
		}
		swg.wgMu.Lock()
		swg.wg.Done()
		swg.wgMu.Unlock()
	}()

	type compileDialogResult struct {
		Graph map[uuid.UUID]*models.DialogNode
		Error error
	}

	compileDialogChannel := make(chan compileDialogResult)
	go func() {
		fmt.Println("Compiling dialog and graph")
		items := []models.ProjectItem(projectItems)
		graph, err := compile.Dialog(redisWriter, &items, publishID)
		result := compileDialogResult{graph, err}
		compileDialogChannel <- result
	}()

	compileMetadataChannel := make(chan error)
	go func() {
		project := models.Project{}
		err = db.DBMap.SelectOne(&project, `SELECT * FROM workbench_projects WHERE "ID"=$1`, projectID)
		if err != nil {
			compileMetadataChannel <- err
			return
		}
		items := []models.ProjectItem(projectItems)
		err := compile.Metadata(redisWriter, project, &items, version, publishID, isDemo)
		compileMetadataChannel <- err
	}()

	compileActorChannel := make(chan error)
	go func() {
		fmt.Println("Compiling actors into zones")
		items := []models.ProjectItem(projectItems)
		err := compile.Actor(redisWriter, &items, publishID)
		compileActorChannel <- err
	}()

	compileTriggerChannel := make(chan error)
	go func() {
		fmt.Println("Compiling triggers into zones")
		triggerItems := []models.ProjectTriggerItem(project.TriggerData)
		err := compile.Trigger(redisWriter, &triggerItems, publishID)
		compileTriggerChannel <- err
	}()

	for i := 0; i < 4; i++ {
		select {
		case msgDialog := <-compileDialogChannel:
			if msgDialog.Error != nil {
				fmt.Println("There was a problem compiling/saving the dialog", msgDialog.Error)
				return msgDialog.Error
			}
			fmt.Println("Successfully compiled and stored dialog graph")

		case msgMetadata := <-compileMetadataChannel:
			if msgMetadata != nil {
				fmt.Println("There was a problem compiling the metadata", msgMetadata)
				return msgMetadata
			}
			fmt.Println("Successfully compiled metadata")

		case msgActor := <-compileActorChannel:
			if msgActor != nil {
				fmt.Println("There was a problem compiling the actors", msgActor)
				return msgActor
			}
			fmt.Println("Successfully compiled actors")

		case msgTrigger := <-compileTriggerChannel:
			if msgTrigger != nil {
				fmt.Println("There was a problem compiling the triggers", msgTrigger)
				return msgTrigger
			}
			fmt.Println("Successfully compiled triggers")

		}
	}

	redisWriter <- common.RedisCommand{
		Key: "KILL",
	}
	// This ensures that all Redis commands complete execution before closing out
	swg.wgMu.Lock()
	swg.wgSema = 1
	swg.wgMu.Unlock()
	swg.wg.Wait()

	if !isDemo {
		_, err = db.Instance.Exec(`DELETE FROM workbench_projects_needing_review WHERE "ProjectID"=$1`, projectID)
		if err != nil {
			return err
		}
		team := models.Team{}
		err = db.DBMap.SelectOne(&team, `
		SELECT DISTINCT
			p."TeamID" "ID"
		FROM workbench_projects p
		WHERE p."ID"=$1
	`, projectID)
		if err != nil {
			return err
		}

		_, err = db.Instance.Exec(`INSERT INTO published_workbench_projects ("ProjectID", "TeamID") VALUES ($1, $2)`, projectID, team.ID)
		if err != nil {
			return err
		}
	}

	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(publishID), "status"),
		[]byte(fmt.Sprintf("%v", models.PublishStatusPublished))).Exec(redis.Instance)

	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(publishID), "pubtime"),
		[]byte(fmt.Sprintf("%v", time.Now().UnixNano()))).Exec(redis.Instance)

	return nil
}

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis"

	"github.com/artificial-universe-maker/lakshmi/compile"

	"github.com/artificial-universe-maker/core/common"
	"github.com/artificial-universe-maker/core/db"
	"github.com/artificial-universe-maker/core/models"
	"github.com/artificial-universe-maker/core/myerrors"
	"github.com/artificial-universe-maker/core/providers"
)

func main() {
	http.HandleFunc("/", processRequest)
	log.Println("Lakshmi starting server on localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

func processRequest(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	projectID, err := strconv.ParseUint(r.Form.Get("project-id"), 10, 64)

	if err != nil {
		myerrors.Respond(w, &myerrors.MySimpleError{
			Code:    http.StatusBadRequest,
			Log:     err.Error(),
			Req:     r,
			Message: "Invalid project-id",
		})
		return
	}

	redis, err := providers.ConnectRedis()
	if err != nil {
		myerrors.Respond(w, &myerrors.MySimpleError{
			Code: http.StatusInternalServerError,
			Log:  err.Error(),
			Req:  r,
		})
		return
	}
	defer redis.Close()

	err = initiateCompiler(projectID, redis)
	if err != nil {
		common.RedisSET(
			fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID), "status"),
			[]byte(fmt.Sprintf("%v", models.PublishStatusProblem))).Exec(redis)
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

func initiateCompiler(projectID uint64, redis *redis.Client) error {

	err := db.InitializeDB()
	defer db.Instance.Close()
	if err != nil {
		return err
	}

	if os.Getenv("REDIS_ADDR") == "" {
		os.Setenv("REDIS_ADDR", "127.0.0.1:6379")
		os.Setenv("REDIS_PASSWORD", "")
	}

	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID), "status"),
		[]byte(fmt.Sprintf("%v", models.PublishStatusPublishing)))

	var items []models.ProjectItem
	_, err = db.DBMap.Select(&items, `
		SELECT DISTINCT
			p."ID" "ProjectID",
			p."Title",
			
			z."ID" "ZoneID",

			za."ActorID",
			za."ZoneID",

			d."ID" "DialogID",
			d."ActorID",
			d."EntryInput" "DialogEntry",
			d."AlwaysExec",
			d."Statements",
			d."IsRoot",
			
			dr."ParentNodeID" "ParentDialogID",
			dr."ChildNodeID" "ChildDialogID"

			FROM workbench_projects p
			JOIN workbench_zones z
				ON z."ProjectID" = p."ID"
			JOIN workbench_zones_actors za
				ON za."ZoneID"=z."ID"
			JOIN workbench_dialog_nodes d
				ON d."ActorID"=za."ActorID"
			FULL OUTER JOIN workbench_dialog_nodes_relations dr
				ON dr."ParentNodeID"=d."ID" OR dr."ChildNodeID"=d."ID"
			WHERE p."ID"=$1
		`, projectID)
	if err != nil {
		return err
	}

	// Delete old published data
	membersSlice := redis.SMembers(fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID), "keys"))
	redis.Del(membersSlice.Val()...)

	redisWriter := make(chan common.RedisCommand, 10)
	defer close(redisWriter)

	swg := SyncGroup{}

	trackRedisKeys := true

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
			command.Exec(redis)

			// Track all saved keys so that later we can remove them all in a republish
			if trackRedisKeys {
				common.RedisSADD(fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID), "keys"), []byte(command.Key)).Exec(redis)
			}
			swg.wg.Done()
			swg.wgMu.Unlock()
		}
		swg.wgMu.Lock()
		swg.wg.Done()
		swg.wgMu.Unlock()
	}()

	type compileDialogResult struct {
		Graph map[uint64]*models.AumDialogNode
		Error error
	}

	compileDialogChannel := make(chan compileDialogResult)
	go func() {
		fmt.Println("Compiling dialog and graph")
		graph, err := compile.Dialog(redisWriter, &items)
		result := compileDialogResult{graph, err}
		compileDialogChannel <- result
	}()

	compileMetadataChannel := make(chan error)
	go func() {
		project := models.AumProject{}
		err = db.DBMap.SelectOne(&project, `SELECT * FROM workbench_projects WHERE "ID"=$1`, projectID)
		if err != nil {
			compileMetadataChannel <- err
			return
		}
		err := compile.Metadata(redisWriter, project, &items)
		compileMetadataChannel <- err
	}()

	compileActorChannel := make(chan error)
	go func() {
		fmt.Println("Compiling actors into zones")
		err := compile.Actor(redisWriter, &items)
		compileActorChannel <- err
	}()

	compileTriggerChannel := make(chan error)
	go func() {
		fmt.Println("Compiling triggers into zones")
		var items []models.ProjectTriggerItem
		_, err = db.DBMap.Select(&items, `
			SELECT DISTINCT
				p."ID" "ProjectID",
				z."ID" "ZoneID",
				t."TriggerType",
				t."AlwaysExec",
				t."Statements"

			FROM workbench_projects p
			JOIN workbench_zones z
				ON z."ProjectID" = p."ID"
			JOIN workbench_triggers t
				ON t."ZoneID"=z."ID"
			WHERE p."ID"=$1
			`, projectID)
		if err != nil {
			compileTriggerChannel <- err
			return
		}
		err := compile.Trigger(redisWriter, &items)
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

	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID), "status"),
		[]byte(fmt.Sprintf("%v", models.PublishStatusPublished))).Exec(redis)
	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID), "pubtime"),
		[]byte(fmt.Sprintf("%v", time.Now().UnixNano()))).Exec(redis)

	return nil
}

package routes

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/artificial-universe-maker/core/common"
	"github.com/artificial-universe-maker/core/db"

	"github.com/artificial-universe-maker/core/models"
	"github.com/artificial-universe-maker/core/myerrors"
	"github.com/artificial-universe-maker/core/prehandle"
	"github.com/artificial-universe-maker/core/redis"
	"github.com/artificial-universe-maker/core/router"
	uuid "github.com/artificial-universe-maker/go.uuid"
	"github.com/gorilla/mux"
)

// PostSubmit router.Route
// Path: "/user/register",
// Method: "POST",
// Accepts models.TokenValidate
// Responds with status of success or failure
var PostSubmit = &router.Route{
	Path:       "/v1/submit/{id}",
	Method:     "POST",
	Handler:    http.HandlerFunc(postSubmitHandler),
	Prehandler: []prehandle.Prehandler{prehandle.SetJSON},
}

// AIRequestHandler handles requests that expect language parsing and an AI response
// Currently expects ApiAi requests
// This is the core functionality of Brahman, which routes to appropriate IntentHandlers
func postSubmitHandler(w http.ResponseWriter, r *http.Request) {

	urlparams := mux.Vars(r)

	projectID, err := uuid.FromString(urlparams["id"])
	if err != nil {
		myerrors.Respond(w, &myerrors.MySimpleError{
			Code:    http.StatusBadRequest,
			Message: "bad_id",
			Req:     r,
		})
		return
	}

	submitQuery := `
		INSERT INTO static_published_projects_versioned
			("ProjectID", "Version", "Title", "Category", "Tags", "ProjectData")
		SELECT
			$1 "ProjectID", $2 "Version", p."Title", p."Category", p."Tags", json_agg(t) "ProjectData"
		FROM
		(SELECT DISTINCT
			za."ActorID",
			za."ZoneID",

			d."ID" "DialogID",
			d."EntryInput" "DialogEntry",
			d."AlwaysExec",
			d."Statements",
			d."IsRoot",
			d."UnknownHandler",
			
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
			WHERE p."ID"=$1) t,
		(SELECT
			"Title",
			"Category",
			"Tags"
			FROM workbench_projects
			WHERE "ID"=$1 LIMIT 1) p
		GROUP BY (p."Title", p."Category", p."Tags")
	`

	// TODO: Gracefully handle PublishStatusProblem

	var currentVersion int64
	status, _ := strconv.ParseInt(redis.Instance.Get(fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status")).Val(), 10, 8)
	currentPublishStatus := models.PublishStatus(status)
	if currentPublishStatus == models.PublishStatusPublishing {
		return
	} else if currentPublishStatus == models.PublishStatusDenied ||
		currentPublishStatus == models.PublishStatusUnderReview {
		currentVersion, err = redis.Instance.HGet(models.KeynavProjectMetadataStatic(projectID.String()), "version").Int64()
		if err != nil {
			myerrors.ServerError(w, r, err)
			return
		}
		db.Instance.Exec(`DELETE FROM static_published_projects_versioned WHERE "Version"=$1 AND "ProjectID"=$2`, currentVersion, projectID.String())
		db.Instance.Exec(submitQuery, projectID.String(), currentVersion)
		return
	} else if currentPublishStatus == models.PublishStatusProblem {
		// TODO: Handle this better
		fmt.Println("There's an issue")
		return
	}

	currentVersion = redis.Instance.HIncrBy(models.KeynavProjectMetadataStatic(projectID.String()), "version", 1).Val()
	trimVersions := currentVersion - 3
	db.Instance.Exec(`DELETE FROM static_published_projects_versioned WHERE "Version"<=$1 AND "ProjectID"=$2`, trimVersions, projectID.String())

	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
		[]byte(fmt.Sprintf("%v", models.PublishStatusPublishing))).Exec(redis.Instance)

	db.Instance.Exec(submitQuery, projectID.String(), currentVersion)

	// TODO: This should probably never need to run
	_, err = db.Instance.Exec(`DELETE FROM workbench_projects_needing_review WHERE "ProjectID"=$1`, projectID)
	if err != nil {
		myerrors.ServerError(w, r, err)
		return
	}
	_, err = db.Instance.Exec(`INSERT INTO workbench_projects_needing_review ("ProjectID") VALUES ($1)`, projectID)
	if err != nil {
		myerrors.ServerError(w, r, err)
		return
	}

	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
		[]byte(fmt.Sprintf("%v", models.PublishStatusUnderReview))).Exec(redis.Instance)

}

package routes

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/talkative-ai/core/common"
	"github.com/talkative-ai/core/db"
	"github.com/talkative-ai/lakshmi/helpers"

	"github.com/gorilla/mux"
	"github.com/talkative-ai/core/models"
	"github.com/talkative-ai/core/myerrors"
	"github.com/talkative-ai/core/prehandle"
	"github.com/talkative-ai/core/redis"
	"github.com/talkative-ai/core/router"
	uuid "github.com/talkative-ai/go.uuid"
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

	// TODO: Gracefully handle PublishStatusProblem

	var currentVersion int64
	status, _ := strconv.ParseInt(redis.Instance.Get(fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status")).Val(), 10, 8)
	currentPublishStatus := models.PublishStatus(status)
	tx, err := db.Instance.Begin()
	if err != nil {
		common.RedisSET(
			fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
			[]byte(fmt.Sprintf("%v", models.PublishStatusProblem))).Exec(redis.Instance)
		myerrors.ServerError(w, r, err)
		return
	}
	if currentPublishStatus == models.PublishStatusPublishing {
		return
	} else if currentPublishStatus == models.PublishStatusDenied ||
		currentPublishStatus == models.PublishStatusUnderReview {
		currentVersion, err = redis.Instance.HGet(models.KeynavProjectMetadataStatic(projectID.String()), "version").Int64()
		if err != nil {
			common.RedisSET(
				fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
				[]byte(fmt.Sprintf("%v", models.PublishStatusProblem))).Exec(redis.Instance)
			myerrors.ServerError(w, r, err)
			return
		}
		tx.Exec(`DELETE FROM static_published_projects_versioned WHERE "Version"=$1 AND "ProjectID"=$2`, currentVersion, projectID.String())
	} else if currentPublishStatus == models.PublishStatusProblem {
		if err != nil {
			common.RedisSET(
				fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
				[]byte(fmt.Sprintf("%v", models.PublishStatusProblem))).Exec(redis.Instance)
			myerrors.ServerError(w, r, err)
			return
		}
	} else {
		currentVersion = redis.Instance.HIncrBy(models.KeynavProjectMetadataStatic(projectID.String()), "version", 1).Val()
		trimVersions := currentVersion - 3
		db.Instance.Exec(`DELETE FROM static_published_projects_versioned WHERE "Version"<=$1 AND "ProjectID"=$2`, trimVersions, projectID.String())
	}

	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
		[]byte(fmt.Sprintf("%v", models.PublishStatusPublishing))).Exec(redis.Instance)
	helpers.CreateVersionedProject(tx, projectID.String(), currentVersion)
	tx.Exec(`DELETE FROM workbench_projects_needing_review WHERE "ProjectID"=$1`, projectID)
	tx.Exec(`INSERT INTO workbench_projects_needing_review ("ProjectID") VALUES ($1)`, projectID)
	err = tx.Commit()
	if err != nil {
		common.RedisSET(
			fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
			[]byte(fmt.Sprintf("%v", models.PublishStatusProblem))).Exec(redis.Instance)
		common.RedisSET(
			fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
			[]byte(fmt.Sprintf("%v", models.PublishStatusProblem))).Exec(redis.Instance)
		myerrors.ServerError(w, r, err)
		return
	}
	common.RedisSET(
		fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(projectID.String()), "status"),
		[]byte(fmt.Sprintf("%v", models.PublishStatusUnderReview))).Exec(redis.Instance)
}

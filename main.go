package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/artificial-universe-maker/lakshmi/compile"

	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/db"
	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/go-utilities/myerrors"
	"github.com/artificial-universe-maker/go-utilities/providers"
)

func main() {
	http.HandleFunc("/", processRequest)
	http.ListenAndServe(":8080", nil)
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

	initiateCompiler(projectID)
}

func initiateCompiler(projectID uint64) error {

	err := db.InitializeDB()
	if err != nil {
		return err
	}

	var items []common.ProjectItem
	_, err = db.DBMap.Select(&items, `
		SELECT DISTINCT
			p."ID" ProjectID,
			p."Title",
			
			z."ID" ZoneID,

			za."ActorID",
			za."ZoneID",

			d."ID" DialogID,
			d."ActorID",
			d."Entry" DialogEntry,
			d."LogicalSetID",

			ls."ID" LogicalSetID,
			ls."Always" LogicalSetAlways,
			ls."Statements" LogicalSetStatements,
			
			dr."ParentNodeID" ParentDialogID,
			dr."ChildNodeID" ChildDialogID

		FROM
			workbench_projects p,
			workbench_zones z,
			workbench_dialog_nodes d,
			workbench_dialog_nodes_relations dr,
			workbench_logical_set ls,
			workbench_zones_actors za
		WHERE
			(p."ID"=$1 AND z."ID"=p."ID" AND za."ZoneID"=z."ID" AND d."ActorID"=za."ActorID" AND ls."ID" = d."LogicalSetID")
			AND (dr."ParentNodeID" = d."ID" OR d."ID" = dr."ChildNodeID")
		`, projectID)
	if err != nil {
		return err
	}

	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("REDIS_PASSWORD", "")
	redis, err := providers.ConnectRedis()
	if err != nil {
		return err
	}
	defer redis.Close()

	redisWriter := make(chan common.RedisCommand)
	defer close(redisWriter)

	wg := sync.WaitGroup{}
	go func() {
		for command := range redisWriter {
			wg.Add(1)
			command(redis)
			wg.Done()
		}
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
		err = db.DBMap.SelectOne(&project, `SELECT * FROM workbench_projects WHERE id=$1`, projectID)
		if err != nil {
			compileMetadataChannel <- err
		}
		err := compile.Metadata(redisWriter, project)
		compileMetadataChannel <- err
	}()

	for i := 0; i < 2; i++ {
		select {
		case msgDialog := <-compileDialogChannel:
			if msgDialog.Error != nil {
				fmt.Println("There was a problem compiling/saving the dialog", msgDialog.Error)
				return msgDialog.Error
			}
			fmt.Println("Successfully compiled and stored dialog graph")

		case msgMetadata := <-compileMetadataChannel:
			if msgMetadata != nil {
				fmt.Println("There was a problem saving the metadata", msgMetadata)
				return msgMetadata
			}
			fmt.Println("Successfully saved metadata")
		}
	}

	wg.Wait()

	return nil
}

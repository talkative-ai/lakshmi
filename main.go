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
		SELECT
			p.id ProjectID,
			p.title,
			
			z.id ZoneID,
			
			d.id DialogID,
			d.zone_id ZoneID,
			d.entry DialogEntry,
			d.logical_set_id LogicalSetID,

			ls.id LogicalSetID,
			ls.always LogicalSetAlways,
			ls.statements LogicalSetStatements,
			
			dr.parent_node_id ParentDialogID,
			dr.child_node_id ChildDialogID

		FROM
			projects p,
			zones z,
			dialog_nodes d,
			dialog_nodes_relations dr,
			logical_set ls
		WHERE
			(p.id=$1 AND z.id=p.id AND d.zone_id=z.id AND ls.id = d.logical_set_id)
			AND (dr.parent_node_id = d.id OR d.id = dr.child_node_id)
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

	redisWriter := make(chan common.RedisBytes)
	defer close(redisWriter)

	wg := sync.WaitGroup{}
	go func() {
		for v := range redisWriter {
			wg.Add(1)
			redis.Set(v.Key, v.Bytes, 0)
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
		graph, err := compile.CompileDialog(redisWriter, &items)
		result := compileDialogResult{graph, err}
		compileDialogChannel <- result
	}()

	compileMetadataChannel := make(chan error)

	go func() {
		project := models.AumProject{}
		err = db.DBMap.SelectOne(&project, `SELECT * FROM projects WHERE id=$1`, projectID)
		if err != nil {
			compileMetadataChannel <- err
		}
		err := compile.CompileMetadata(redisWriter, project)
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

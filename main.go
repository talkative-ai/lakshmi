package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/artificial-universe-maker/lakshmi/compile"

	"github.com/artificial-universe-maker/go-utilities/db"
	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/go-utilities/myerrors"
	"github.com/artificial-universe-maker/go-utilities/providers"
	"github.com/artificial-universe-maker/lakshmi/helpers"
)

func main() {
	http.HandleFunc("/", processRequest)
	http.ListenAndServe(":8080", nil)
}

func processRequest(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	project_id, err := strconv.ParseUint(r.Form.Get("project-id"), 10, 64)

	if err != nil {
		myerrors.Respond(w, &myerrors.MySimpleError{
			Code:    http.StatusBadRequest,
			Log:     err.Error(),
			Req:     r,
			Message: "Invalid project-id",
		})
		return
	}

	initiateCompiler(project_id)
}

func initiateCompiler(project_id uint64) error {

	err := db.InitializeDB()
	if err != nil {
		return err
	}

	type ProjectItem struct {
		ProjectID            uint64
		ZoneID               uint64
		DialogID             uint64
		DialogEntry          string
		ParentDialogID       uint64
		ChildDialogID        uint64
		LogicalSetAlways     string
		LogicalSetStatements sql.NullString
		LogicalSetID         uint64
	}

	var items []ProjectItem
	_, err = db.DBMap.Select(&items, `
		SELECT
			p.id ProjectID,
			
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
		`, project_id)
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

	redisWriter := make(chan helpers.RedisBytes)
	defer close(redisWriter)

	go func() {
		for v := range redisWriter {
			redis.Set(v.Key, v.Bytes, 0)
		}
	}()

	dialogGraph := map[uint64]*models.AumDialogNode{}
	dialogGraphRoots := map[uint64]*bool{}
	dialogEntrySet := map[uint64]map[string]bool{}
	edge := map[uint64]bool{}

	for _, item := range items {

		if _, ok := dialogGraph[item.DialogID]; !ok {
			dialogGraph[item.DialogID] = &models.AumDialogNode{}
			dialogGraph[item.DialogID].EntryInput = []models.AumDialogInput{}
			dialogGraph[item.DialogID].ChildNodes = &[]*models.AumDialogNode{}
			dialogGraph[item.DialogID].ParentNodes = &[]*models.AumDialogNode{}
			dialogGraph[item.DialogID].LogicalSet = models.RawLBlock{}
			dialogGraph[item.DialogID].ID = item.DialogID
			dialogGraph[item.DialogID].ZoneID = item.ZoneID
			dialogGraph[item.DialogID].ProjectID = item.ProjectID
			dialogEntrySet[item.DialogID] = map[string]bool{}
		}

		json.Unmarshal([]byte(item.LogicalSetAlways), &dialogGraph[item.DialogID].LogicalSet.AlwaysExec)
		if ok := dialogEntrySet[item.DialogID][item.DialogEntry]; !ok {
			dialogGraph[item.DialogID].EntryInput = append(dialogGraph[item.DialogID].EntryInput, models.AumDialogInput(item.DialogEntry))
			dialogEntrySet[item.DialogID][item.DialogEntry] = true
		}

		if item.ParentDialogID == item.DialogID {

			if dialogGraphRoots[item.DialogID] == nil {
				v := true
				dialogGraphRoots[item.DialogID] = &v
			}

			c := dialogGraph[item.ChildDialogID]
			if c != nil {
				hasEdge := edge[item.DialogID]
				if !hasEdge {
					appendedChildren := append(*dialogGraph[item.DialogID].ChildNodes, c)
					dialogGraph[item.DialogID].ChildNodes = &appendedChildren
					appendedParents := append(*c.ParentNodes, dialogGraph[item.DialogID])
					c.ParentNodes = &appendedParents
					edge[item.DialogID] = true
					edge[item.ChildDialogID] = true
				}
			}
		} else {
			p := dialogGraph[item.ParentDialogID]
			if p != nil {
				hasEdge := edge[item.DialogID]
				if !hasEdge {
					appendedChildren := append(*dialogGraph[item.DialogID].ParentNodes, p)
					dialogGraph[item.DialogID].ParentNodes = &appendedChildren
					appendedParents := append(*p.ChildNodes, dialogGraph[item.DialogID])
					p.ChildNodes = &appendedParents
					edge[item.DialogID] = true
					edge[item.ParentDialogID] = true
					v := false
					dialogGraphRoots[item.DialogID] = &v
				}
			}
		}
	}

	var wg sync.WaitGroup

	for k, isRoot := range dialogGraphRoots {
		if !*isRoot {
			continue
		}
		wg.Add(1)
		node := *dialogGraph[k]
		go func(node models.AumDialogNode) {
			defer wg.Done()
			compile.CompileDialog(node, redisWriter)
		}(node)
	}

	wg.Wait()

	return nil
}

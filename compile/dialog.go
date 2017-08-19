package compile

import (
	"encoding/json"
	"sync"

	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
)

func CompileDialog(redisWriter chan common.RedisCommand, items *[]common.ProjectItem) (map[uint64]*models.AumDialogNode, error) {

	dialogGraph := map[uint64]*models.AumDialogNode{}
	dialogGraphRoots := map[uint64]*bool{}
	dialogEntrySet := map[uint64]map[string]bool{}
	edge := map[uint64]bool{}

	for _, item := range *items {

		if _, ok := dialogGraph[item.DialogID]; !ok {
			dialogGraph[item.DialogID] = &models.AumDialogNode{}
			dialogGraph[item.DialogID].EntryInput = []models.AumDialogInput{}
			dialogGraph[item.DialogID].LogicalSet = models.RawLBlock{}
			dialogGraph[item.DialogID].ID = item.DialogID
			dialogGraph[item.DialogID].ZoneID = item.ZoneID
			dialogGraph[item.DialogID].ProjectID = item.ProjectID

			dialogGraph[item.DialogID].EntryInput = make([]models.AumDialogInput, len(item.DialogEntry.Val))
			for idx, val := range item.DialogEntry.Val {
				dialogGraph[item.DialogID].EntryInput[idx] = models.AumDialogInput(val)
			}
			dialogEntrySet[item.DialogID] = map[string]bool{}
			json.Unmarshal([]byte(item.LogicalSetAlways), &dialogGraph[item.DialogID].LogicalSet.AlwaysExec)
		}

		if item.ParentDialogID == item.DialogID {

			dialogGraph[item.DialogID].ChildNodes = &[]*models.AumDialogNode{}

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
			dialogGraph[item.DialogID].ParentNodes = &[]*models.AumDialogNode{}

			v := false
			dialogGraphRoots[item.DialogID] = &v

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
				}
			}
		}
	}

	var wg sync.WaitGroup

	for k, isRoot := range dialogGraphRoots {
		if !*isRoot {
			delete(dialogGraph, k)
			continue
		}
		wg.Add(1)
		node := *dialogGraph[k]
		go func(node models.AumDialogNode) {
			defer wg.Done()
			helpers.CompileDialogNode(node, redisWriter)
		}(node)
	}

	wg.Wait()

	return dialogGraph, nil
}

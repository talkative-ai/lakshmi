package compile

import (
	"encoding/json"
	"sync"

	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
)

// Dialog compiles the dialogs into byte slices.
// It takes the SQL rows (cast as ProjectItem structs) ans builds a dialog graph
// Then for each dialog graph root item, it compiles it via the helper DialogNode
// which will finish the compilation process.
// This includes action bundles, logical blocks, and child nodes recursively.
func Dialog(redisWriter chan common.RedisCommand, items *[]common.ProjectItem) (map[uint64]*models.AumDialogNode, error) {

	dialogGraph := map[uint64]*models.AumDialogNode{}
	dialogGraphRoots := map[uint64]*bool{}
	dialogEntrySet := map[uint64]map[string]bool{}
	edge := map[uint64]bool{}

	for _, item := range *items {

		// Many ProjectItems have repeating DialogIDs
		// This is because they're SQL rows from a join query
		if _, ok := dialogGraph[item.DialogID]; !ok {
			dialogGraph[item.DialogID] = &models.AumDialogNode{
				AumModel:   models.AumModel{ID: item.DialogID},
				ActorID:    item.ActorID,
				ProjectID:  item.ProjectID,
				EntryInput: []models.AumDialogInput{},
				LogicalSet: models.RawLBlock{},
			}

			// Here we can convert a string value into an AumDialogInput value
			// as the dialog entry point.
			// This is good for machine learned patterns, such as {greetings} which could match
			// Hello, Hola, Yo, etc.
			// As compatible with api.ai
			// TODO: This is a harder engineering problem.
			// Consider supporting a list of raw text inputs?
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
			// If the current item is a parent dialog item,
			// and the child has dialog has already been processed
			// then we create an edge from the dialog to its child
			// TODO: Handle dialog node cycles (Create a "IsRoot" bool?)
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
			// Same as above, except for a child node
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

	// With our graph constructed, we find go through the root dialog nodes
	// And compile them.
	// helpers.DialogNode will recurse down the children nodes
	for k, isRoot := range dialogGraphRoots {
		if !*isRoot {
			delete(dialogGraph, k)
			continue
		}
		wg.Add(1)
		node := *dialogGraph[k]
		go func(node models.AumDialogNode) {
			defer wg.Done()
			helpers.DialogNode(node, redisWriter)
		}(node)
	}

	wg.Wait()

	return dialogGraph, nil
}

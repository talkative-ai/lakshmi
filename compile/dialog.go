package compile

import (
	"encoding/json"
	"sync"

	"github.com/artificial-universe-maker/core/common"
	"github.com/artificial-universe-maker/core/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
)

// Dialog compiles the dialogs into byte slices.
// It takes the SQL rows (cast as ProjectItem structs) ans builds a dialog graph
// Then for each dialog graph root item, it compiles it via the helper DialogNode
// which will finish the compilation process.
// This includes action bundles, logical blocks, and child nodes recursively.
func Dialog(redisWriter chan common.RedisCommand, items *[]models.ProjectItem) (map[uint64]*models.AumDialogNode, error) {

	dialogGraph := map[uint64]*models.AumDialogNode{}
	dialogGraphRoots := map[uint64]bool{}
	dialogEntrySet := map[uint64]map[string]bool{}
	edgeTo := map[uint64]map[uint64]bool{}

	for _, item := range *items {

		// Many ProjectItems have repeating DialogIDs
		// This is because they're SQL rows from a join query
		if _, ok := dialogGraph[item.DialogID]; !ok {
			isRoot := item.IsRoot
			dialogGraph[item.DialogID] = &models.AumDialogNode{
				AumModel:   models.AumModel{ID: item.DialogID},
				ActorID:    item.ActorID,
				ProjectID:  item.ProjectID,
				EntryInput: []models.AumDialogInput{},
				RawLBlock:  item.RawLBlock,
				IsRoot:     &isRoot,
			}

			edgeTo[item.DialogID] = map[uint64]bool{}

			// Here we can convert a string value into an AumDialogInput value
			// as the dialog entry point.
			// This is good for machine learned patterns, such as {greetings} which could match
			// Hello, Hola, Yo, etc.
			// As compatible with api.ai
			// TODO: This is a harder engineering problem.
			// Supporting a list of raw text inputs for now but upgrade later
			dialogGraph[item.DialogID].EntryInput = make([]models.AumDialogInput, len(item.DialogEntry.Val))
			for idx, val := range item.DialogEntry.Val {
				dialogGraph[item.DialogID].EntryInput[idx] = models.AumDialogInput(val)
			}
			dialogEntrySet[item.DialogID] = map[string]bool{}
			json.Unmarshal([]byte(item.LogicalSetAlways), &dialogGraph[item.DialogID].AlwaysExec)

			if item.IsRoot {
				dialogGraphRoots[item.DialogID] = true
			}
		}

		if item.ParentDialogID.Valid && uint64(item.ParentDialogID.Int64) == item.DialogID {

			if dialogGraph[item.DialogID].ChildNodes == nil {
				dialogGraph[item.DialogID].ChildNodes = &[]*models.AumDialogNode{}
			}

			c := dialogGraph[uint64(item.ChildDialogID.Int64)]
			// If the current item is a parent dialog item,
			// and the child has dialog has already been processed
			// then we create an edge from the dialog to its child
			if c != nil {
				if ok := edgeTo[item.DialogID][uint64(item.ChildDialogID.Int64)]; !ok {
					appendedChildren := append(*dialogGraph[item.DialogID].ChildNodes, c)
					dialogGraph[item.DialogID].ChildNodes = &appendedChildren
					appendedParents := append(*c.ParentNodes, dialogGraph[item.DialogID])
					c.ParentNodes = &appendedParents
					edgeTo[item.DialogID][uint64(item.ChildDialogID.Int64)] = true
				}
			}
		} else if item.ParentDialogID.Valid && item.ChildDialogID.Valid {

			// Same as above, except for a child node
			if dialogGraph[item.DialogID].ParentNodes == nil {
				dialogGraph[item.DialogID].ParentNodes = &[]*models.AumDialogNode{}
			}

			p := dialogGraph[uint64(item.ParentDialogID.Int64)]
			if p != nil {
				if _, ok := edgeTo[uint64(item.ParentDialogID.Int64)]; !ok {
					edgeTo[uint64(item.ParentDialogID.Int64)] = map[uint64]bool{}
				}
				if ok := edgeTo[uint64(item.ParentDialogID.Int64)][item.DialogID]; !ok {
					appendedChildren := append(*dialogGraph[item.DialogID].ParentNodes, p)
					dialogGraph[item.DialogID].ParentNodes = &appendedChildren
					appendedParents := append(*p.ChildNodes, dialogGraph[item.DialogID])
					p.ChildNodes = &appendedParents
					edgeTo[uint64(item.ParentDialogID.Int64)][item.DialogID] = true
				}
			}
		}
	}

	var wg sync.WaitGroup

	// With our graph constructed, we find go through the root dialog nodes
	// And compile them.
	// helpers.DialogNode will recurse down the children nodes
	for rootID := range dialogGraphRoots {
		wg.Add(1)
		node := *dialogGraph[rootID]
		go func(node models.AumDialogNode) {
			defer wg.Done()
			helpers.DialogNode(node, redisWriter, common.SyncMapUint64{})
		}(node)
	}

	wg.Wait()

	return dialogGraph, nil
}

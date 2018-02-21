package compile

import (
	"encoding/json"
	"sync"

	"github.com/talkative-ai/core/common"
	"github.com/talkative-ai/core/models"
	uuid "github.com/talkative-ai/go.uuid"
	"github.com/talkative-ai/lakshmi/helpers"
)

// Dialog compiles the dialogs into byte slices.
// It takes the SQL rows (cast as ProjectItem structs) ans builds a dialog graph
// Then for each dialog graph root item, it compiles it via the helper DialogNode
// which will finish the compilation process.
// This includes action bundles, logical blocks, and child nodes recursively.
func Dialog(redisWriter chan common.RedisCommand, items *[]models.ProjectItem) (map[uuid.UUID]*models.DialogNode, error) {

	dialogGraph := map[uuid.UUID]*models.DialogNode{}
	dialogGraphRoots := map[uuid.UUID]bool{}
	dialogEntrySet := map[uuid.UUID]map[uuid.UUID]bool{}
	edgeTo := map[uuid.UUID]map[uuid.UUID]bool{}

	for _, item := range *items {

		// TODO: Generalize this using reflection and tags somehow
		// Many ProjectItems have repeating DialogIDs
		// This is because they're SQL rows from a join query
		if _, ok := dialogGraph[item.DialogID]; !ok {
			dialogGraph[item.DialogID] = &models.DialogNode{
				Model: models.Model{
					ID: item.DialogID,
				},
				ActorID:        item.ActorID,
				ProjectID:      item.ProjectID,
				EntryInput:     []models.DialogInput{},
				RawLBlock:      item.RawLBlock,
				IsRoot:         item.IsRoot,
				UnknownHandler: item.UnknownHandler,
			}

			edgeTo[item.DialogID] = map[uuid.UUID]bool{}

			// Here we can convert a string value into an DialogInput value
			// as the dialog entry point.
			// This is good for machine learned patterns, such as {greetings} which could match
			// Hello, Hola, Yo, etc.
			// As compatible with api.ai
			// TODO: This is a harder engineering problem.
			// Supporting a list of raw text inputs for now but upgrade later
			dialogGraph[item.DialogID].EntryInput = make([]models.DialogInput, len(item.DialogEntry))
			for idx, val := range item.DialogEntry {
				dialogGraph[item.DialogID].EntryInput[idx] = models.DialogInput(val)
			}
			dialogEntrySet[item.DialogID] = map[uuid.UUID]bool{}
			json.Unmarshal([]byte(item.LogicalSetAlways), &dialogGraph[item.DialogID].AlwaysExec)

			if item.IsRoot {
				dialogGraphRoots[item.DialogID] = true
			}
		}

		if item.ParentDialogID.Valid && item.ParentDialogID.UUID == item.DialogID {

			if dialogGraph[item.DialogID].ChildNodes == nil {
				dialogGraph[item.DialogID].ChildNodes = &[]*models.DialogNode{}
			}

			c := dialogGraph[item.ChildDialogID.UUID]
			if c.ParentNodes == nil {
				c.ParentNodes = &[]*models.DialogNode{}
			}
			// If the current item is a parent dialog item,
			// and the child has dialog has already been processed
			// then we create an edge from the dialog to its child
			if c != nil {
				if ok := edgeTo[item.DialogID][item.ChildDialogID.UUID]; !ok {
					appendedChildren := append(*dialogGraph[item.DialogID].ChildNodes, c)
					dialogGraph[item.DialogID].ChildNodes = &appendedChildren
					appendedParents := append(*c.ParentNodes, dialogGraph[item.DialogID])
					c.ParentNodes = &appendedParents
					edgeTo[item.DialogID][item.ChildDialogID.UUID] = true
				}
			}
		} else if item.ParentDialogID.Valid && item.ChildDialogID.Valid {

			// Same as above, except for a child node
			if dialogGraph[item.DialogID].ParentNodes == nil {
				dialogGraph[item.DialogID].ParentNodes = &[]*models.DialogNode{}
			}

			p := dialogGraph[item.ParentDialogID.UUID]
			// Same as above, except for a child node
			if p.ParentNodes == nil {
				p.ParentNodes = &[]*models.DialogNode{}
			}
			if p != nil {
				if _, ok := edgeTo[item.ParentDialogID.UUID]; !ok {
					edgeTo[item.ParentDialogID.UUID] = map[uuid.UUID]bool{}
				}
				if ok := edgeTo[item.ParentDialogID.UUID][item.DialogID]; !ok {
					appendedChildren := append(*dialogGraph[item.DialogID].ParentNodes, p)
					dialogGraph[item.DialogID].ParentNodes = &appendedChildren
					appendedParents := append(*p.ChildNodes, dialogGraph[item.DialogID])
					p.ChildNodes = &appendedParents
					edgeTo[item.ParentDialogID.UUID][item.DialogID] = true
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
		go func(node models.DialogNode) {
			defer wg.Done()
			helpers.DialogNode(node, redisWriter, common.SyncMapUUID{})
		}(node)
	}

	wg.Wait()

	return dialogGraph, nil
}

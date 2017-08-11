package compile

import (
	"fmt"
	"sync"

	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
	"github.com/artificial-universe-maker/lakshmi/prepare"
)

func CompileDialog(id int, dialog models.AumDialog) {
	ch := make(chan helpers.BSliceIndex)
	processedLBlocks := make([]models.LBlock, len(dialog.Nodes))
	wg := sync.WaitGroup{}
	for i, node := range dialog.Nodes {

		// Bundle the AlwaysExec
		go prepare.BundleActions(i, node.LogicalSet.AlwaysExec, ch)

		// Prepare to bundle Exec actions within LStatements
		processedLBlocks[i] = models.LBlock{}
		stmt := make([][]models.LStatement, len(*node.LogicalSet.Statements))
		processedLBlocks[i].Statements = &stmt

		for j, OrBlock := range *node.LogicalSet.Statements {
			(*processedLBlocks[i].Statements)[j] = make([]models.LStatement, len(OrBlock))
			for k, AndBlock := range OrBlock {
				wg.Add(1)
				go func(idx1, idx2 int, AndBlock models.RawLStatement) {
					cinner := make(chan helpers.BSliceIndex)
					go prepare.BundleActions(0, AndBlock.Exec, cinner)
					// Write bundled := (<-cinner).Bslice to redis
					close(cinner)
					redisKey := fmt.Sprintf("compiled:1:entities:1:%i:%i", 1, 1)
					(*processedLBlocks[i].Statements)[idx1][idx2] = models.LStatement{Operators: AndBlock.Operators, Exec: redisKey}
					wg.Done()
				}(j, k, AndBlock)
			}
		}
	}

	chcount := 0
	for bundled := range ch {
		//	compiled:{pub_id}:entities:{Dialogs}:{dialog_id}:{action_set}
		redisKey := fmt.Sprintf("compiled:1:entities:1:%i:%i", 1, 1)
		// Write bundled to redis
		processedLBlocks[bundled.Index].AlwaysExec = redisKey
		chcount++
		if chcount == len(dialog.Nodes) {
			close(ch)
		}
	}

	wg.Wait()

	for _, block := range processedLBlocks {
		helpers.CompileLogic(&block)
	}
}

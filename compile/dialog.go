package compile

import (
	"fmt"
	"sync"

	"github.com/artificial-universe-maker/go-utilities/keynav"
	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
	"github.com/artificial-universe-maker/lakshmi/prepare"
)

func compileNodeHelper(idx int, node models.AumDialogNode, redisWriter chan helpers.RedisBytes) []byte {
	lblock := models.LBlock{}

	wg := sync.WaitGroup{}

	// Bundle the AlwaysExec actions
	wg.Add(1)
	go func() {
		defer wg.Done()

		bslice := prepare.BundleActions(node.LogicalSet.AlwaysExec)
		key := keynav.CompiledEntities(1, models.AEIDActionBundle, fmt.Sprintf("%v", idx))
		redisWriter <- helpers.RedisBytes{
			Key:   key,
			Bytes: bslice,
		}
		lblock.AlwaysExec = key
	}()

	if node.LogicalSet.Statements == nil {
		wg.Wait()
		return helpers.CompileLogic(&lblock)
	}

	// Prepare an array for blocks of processed "if/elif/else" blocks
	stmt := make([][]models.LStatement, len(*node.LogicalSet.Statements))
	lblock.Statements = &stmt

	// Iterate through conditional statements
	for j, Statements := range *node.LogicalSet.Statements {

		// Prepare an array for individual processed "if/elif/else" blocks
		(*lblock.Statements)[j] = make([]models.LStatement, len(Statements))

		// Iterate through statements containing conditional logic
		// Each "Statement" here represents an individual if/elif/else block
		for k, Statement := range Statements {
			wg.Add(1)
			go func(idx1, idx2 int, Statement models.RawLStatement) {
				defer wg.Done()
				bslice := prepare.BundleActions(Statement.Exec)

				key := keynav.CompiledEntities(1, models.AEIDActionBundle, fmt.Sprintf("%vx%vx%v", idx, idx1, idx2))

				redisWriter <- helpers.RedisBytes{
					Key:   key,
					Bytes: bslice,
				}
				(*lblock.Statements)[idx1][idx2] = models.LStatement{Operators: Statement.Operators, Exec: key}
			}(j, k, Statement)
		}
	}

	wg.Wait()
	return helpers.CompileLogic(&lblock)
}

func CompileDialog(dialog models.AumDialog, redisWriter chan helpers.RedisBytes) {
	wg := sync.WaitGroup{}
	wg.Add(len(dialog.Nodes))
	for i, node := range dialog.Nodes {
		go func(i int, node models.AumDialogNode) {
			wg.Done()
			compiledNode := compileNodeHelper(i, node, redisWriter)
			key := keynav.CompiledEntities(1, models.AEIDDialogNode, fmt.Sprintf("%v", i))

			redisWriter <- helpers.RedisBytes{
				Key:   key,
				Bytes: compiledNode,
			}
		}(i, node)
	}

	wg.Wait()
}

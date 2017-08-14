package compile

import (
	"encoding/binary"
	"sync"
	"sync/atomic"

	"github.com/artificial-universe-maker/go-utilities/keynav"
	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
	"github.com/artificial-universe-maker/lakshmi/prepare"
)

func compileNodeHelper(node models.AumDialogNode, redisWriter chan helpers.RedisBytes) []byte {
	lblock := models.LBlock{}

	wg := sync.WaitGroup{}
	bundleCount := uint64(0)

	// Bundle the AlwaysExec actions
	wg.Add(1)
	go func() {
		defer wg.Done()

		bslice := prepare.BundleActions(node.LogicalSet.AlwaysExec)
		key := keynav.CompiledDialogNodeActionBundle(node.ProjectID, node.ID, atomic.AddUint64(&bundleCount, 1)-1)
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

				key := keynav.CompiledDialogNodeActionBundle(node.ProjectID, node.ID, atomic.AddUint64(&bundleCount, 1)-1)

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

func CompileDialog(node models.AumDialogNode, redisWriter chan helpers.RedisBytes) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(node models.AumDialogNode) {
		defer wg.Done()

		bslice := []byte{}

		// Append the number of child nodes
		if node.ChildNodes == nil {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(0))
			bslice = append(bslice, b...)
		} else {
			bslice = append(bslice, byte(len(*node.ChildNodes)))

			// Append every child node ID
			for _, child := range *node.ChildNodes {
				b := make([]byte, 8)
				binary.LittleEndian.PutUint64(b, child.ID)
				bslice = append(bslice, b...)
			}
		}

		bslice = append(bslice, compileNodeHelper(node, redisWriter)...)
		compiledKey := keynav.CompiledEntity(node.ProjectID, models.AEIDDialogNode, node.ID)

		redisWriter <- helpers.RedisBytes{
			Key:   compiledKey,
			Bytes: bslice,
		}

		for _, input := range node.EntryInput {
			if node.ParentNodes == nil {
				key := keynav.CompiledDialogRootWithinZone(node.ProjectID, node.ZoneID, string(input))
				redisWriter <- helpers.RedisBytes{
					Key:   key,
					Bytes: []byte(compiledKey),
				}
			} else {
				for _, parent := range *node.ParentNodes {
					key := keynav.CompiledDialogNodeWithinZone(node.ProjectID, node.ZoneID, parent.ID, string(input))
					redisWriter <- helpers.RedisBytes{
						Key:   key,
						Bytes: []byte(compiledKey),
					}
				}
			}
		}

	}(node)

	if node.ChildNodes == nil {
		wg.Wait()
		return
	}

	wg.Add(len(*node.ChildNodes))
	for _, child := range *node.ChildNodes {
		go func(node models.AumDialogNode) {
			defer wg.Done()
			CompileDialog(node, redisWriter)
		}(*child)
	}
	wg.Wait()
}

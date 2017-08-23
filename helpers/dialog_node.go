package helpers

import (
	"sync"
	"sync/atomic"

	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/keynav"
	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/prepare"
)

// compileNodeHelper relates to compiling the node.LogicalSet and the actions therein
// It does this in the following steps
//
// 1. Bundle the AlwaysExec actions and store the key to the Action Bundle in the lblock
//
// 2. 1.	If the LogicalSet has no statements, then convert the new lblock into binary
//				(In this case that just means storing the length of the Action Bundle key)
//				And return the value to the calling function "DialogNode"
//
// 2. 2. 	Else prepare to compile the LogicalSet statements after bundling their actions
//				therein.
//
// 3. Every value accessed in node.LogicalSet will now be mirrored in the lblock variable
//		This is because lblock node.LogicalSet compiled
//
// 4. Iterate through the entire array of node.LogicalSet.Statements,
//		which is an array of arrays of statements
//
// 5. For each statement in each array of statements, bundle the actions
//
// 6. Finally send it all off to be converted to bytes,
//		and return the value to the calling function "DialogNode"
func compileNodeHelper(node models.AumDialogNode, redisWriter chan common.RedisCommand) []byte {
	lblock := models.LBlock{}

	wg := sync.WaitGroup{}

	// For tracking the action bundle ID
	// It will always be a child of the unique node, therefore we can start with a zero ID
	bundleCount := uint64(0)

	// 1. Bundle the AlwaysExec actions and store the key to the Action Bundle in the lblock
	wg.Add(1)
	go func() {
		defer wg.Done()
		bslice := prepare.BundleActions(node.LogicalSet.AlwaysExec)
		key := keynav.CompiledDialogNodeActionBundle(node.ProjectID, node.ID, atomic.AddUint64(&bundleCount, 1)-1)
		redisWriter <- common.RedisSET(key, bslice)
		lblock.AlwaysExec = key
	}()

	// 2. 1.	If the LogicalSet has no statements, then convert the new lblock into binary
	//				In this case that just means storing the length of the Action Bundle key
	if node.LogicalSet.Statements == nil {
		wg.Wait()
		return CompileLogic(&lblock)
	}

	// 2. 2. 	Else prepare to compile the LogicalSet statements after bundling their actions therein
	stmt := make([][]models.LStatement, len(*node.LogicalSet.Statements))
	// 3. Every value accessed in node.LogicalSet will now be mirrored in the lblock variable
	//		This is because lblock node.LogicalSet compiled
	lblock.Statements = &stmt

	// 4. Iterate through the entire array of node.LogicalSet.Statements, which is an array of arrays of statements
	for j, Statements := range *node.LogicalSet.Statements {

		// Prepare an array for individual processed "if/elif/else" blocks
		// Again, as per note 3:
		// 3. Every value accessed in node.LogicalSet will now be mirrored in the lblock variable
		//		This is because lblock node.LogicalSet compiled
		(*lblock.Statements)[j] = make([]models.LStatement, len(Statements))

		// 5. For each statement in each array of statements, bundle the actions
		// Each "Statement" here represents an individual if/elif/else block
		for k, Statement := range Statements {
			wg.Add(1)
			go func(idx1, idx2 int, Statement models.RawLStatement) {
				defer wg.Done()
				bslice := prepare.BundleActions(Statement.Exec)

				key := keynav.CompiledDialogNodeActionBundle(node.ProjectID, node.ID, atomic.AddUint64(&bundleCount, 1)-1)

				redisWriter <- common.RedisSET(key, bslice)
				// Again, as per note 3:
				// 3. Every value accessed in node.LogicalSet will now be mirrored in the lblock variable
				//		This is because lblock node.LogicalSet compiled
				(*lblock.Statements)[idx1][idx2] = models.LStatement{Operators: Statement.Operators, Exec: key}
			}(j, k, Statement)
		}
	}

	wg.Wait()
	return CompileLogic(&lblock)
}

// DialogNode is a helper function to compile.Dialog
// It compiles the node logical blocks, action bundles therein,
// and its child nodes recursively.
func DialogNode(node models.AumDialogNode, redisWriter chan common.RedisCommand) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(node models.AumDialogNode) {
		defer wg.Done()

		bslice := []byte{}

		// Boolean flag whether dialog continues or ends
		if node.ChildNodes == nil {
			bslice = append(bslice, 0)
		} else {
			bslice = append(bslice, 1)
		}

		// Save the compiled logical blocks and action bundles
		bslice = append(bslice, compileNodeHelper(node, redisWriter)...)
		compiledKey := keynav.CompiledEntity(node.ProjectID, models.AEIDDialogNode, node.ID)

		// Send it to be written to Redis
		redisWriter <- common.RedisSET(compiledKey, bslice)

		// Creating a Redis hash which maps every user input to the node's key
		// This enables Brahman to match a user input to the node data while remaining normalized
		for _, input := range node.EntryInput {
			// TODO: This should be a boolean indicator designating root status
			// The reason being that root nodes may be recursively referenced by child nodes
			// For example, a point in the dialog leads back up the dialog tree
			if node.ParentNodes == nil {
				key := keynav.CompiledDialogRootWithinZone(node.ProjectID, node.ZoneID)
				redisWriter <- common.RedisHSET(key, string(input), []byte(compiledKey))
			} else {
				for _, parent := range *node.ParentNodes {
					key := keynav.CompiledDialogNodeWithinZone(node.ProjectID, node.ZoneID, parent.ID)
					redisWriter <- common.RedisHSET(key, string(input), []byte(compiledKey))
				}
			}
		}

	}(node)

	if node.ChildNodes == nil {
		wg.Wait()
		return
	}

	// For every child node, recurse this operation
	wg.Add(len(*node.ChildNodes))
	for _, child := range *node.ChildNodes {
		go func(node models.AumDialogNode) {
			defer wg.Done()
			DialogNode(node, redisWriter)
		}(*child)
	}
	wg.Wait()
}

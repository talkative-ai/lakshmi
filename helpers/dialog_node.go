package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/talkative-ai/snips-nlu-types"

	"github.com/talkative-ai/core/common"
	"github.com/talkative-ai/core/models"
	uuid "github.com/talkative-ai/go.uuid"
	"github.com/talkative-ai/lakshmi/prepare"
)

// compileNodeHelper relates to compiling the node.RawLBlock and the actions therein
// It does this in the following steps
//
// 1. Bundle the AlwaysExec actions and store the key to the Action Bundle in the lblock
//
// 2. 1.	If the.RawLBlock has no statements, then convert the new lblock into binary
//				(In this case that just means storing the length of the Action Bundle key)
//				And return the value to the calling function "DialogNode"
//
// 2. 2. 	Else prepare to compile the.RawLBlock statements after bundling their actions
//				therein.
//
// 3. Every value accessed in node.RawLBlock will now be mirrored in the lblock variable
//		This is because lblock node.RawLBlock compiled
//
// 4. Iterate through the entire array of node.RawLBlock.Statements,
//		which is an array of arrays of statements
//
// 5. For each statement in each array of statements, bundle the actions
//
// 6. Finally send it all off to be converted to bytes,
//		and return the value to the calling function "DialogNode"
func compileNodeHelper(node models.DialogNode, redisWriter chan common.RedisCommand, publishID string) []byte {
	lblock := models.LBlock{}

	wg := sync.WaitGroup{}

	// For tracking the action bundle ID
	// It will always be a child of the unique node, therefore we can start with a zero ID
	bundleCount := uint64(0)

	// 1. Bundle the AlwaysExec actions and store the key to the Action Bundle in the lblock
	wg.Add(1)
	go func() {
		defer wg.Done()
		bslice := prepare.BundleActions(node.RawLBlock.AlwaysExec)
		key := models.KeynavCompiledDialogNodeActionBundle(publishID, node.ID.String(), atomic.AddUint64(&bundleCount, 1)-1)
		redisWriter <- common.RedisSET(key, bslice)
		lblock.AlwaysExec = key
	}()

	// 2. 1.	If the.RawLBlock has no statements, then convert the new lblock into binary
	//				In this case that just means storing the length of the Action Bundle key
	if node.RawLBlock.Statements == nil {
		wg.Wait()
		return CompileLogic(&lblock)
	}

	// 2. 2. 	Else prepare to compile the.RawLBlock statements after bundling their actions therein
	stmt := make([][]models.LStatement, len(*node.RawLBlock.Statements))
	// 3. Every value accessed in node.RawLBlock will now be mirrored in the lblock variable
	//		This is because lblock node.RawLBlock compiled
	lblock.Statements = &stmt

	// 4. Iterate through the entire array of node.RawLBlock.Statements, which is an array of arrays of statements
	for j, Statements := range *node.RawLBlock.Statements {

		// Prepare an array for individual processed "if/elif/else" blocks
		// Again, as per note 3:
		// 3. Every value accessed in node.RawLBlock will now be mirrored in the lblock variable
		//		This is because lblock node.RawLBlock compiled
		(*lblock.Statements)[j] = make([]models.LStatement, len(Statements))

		// 5. For each statement in each array of statements, bundle the actions
		// Each "Statement" here represents an individual if/elif/else block
		for k, Statement := range Statements {
			wg.Add(1)
			go func(idx1, idx2 int, Statement models.RawLStatement) {
				defer wg.Done()
				bslice := prepare.BundleActions(Statement.Exec)

				key := models.KeynavCompiledDialogNodeActionBundle(publishID, node.ID.String(), atomic.AddUint64(&bundleCount, 1)-1)

				redisWriter <- common.RedisSET(key, bslice)
				// Again, as per note 3:
				// 3. Every value accessed in node.RawLBlock will now be mirrored in the lblock variable
				//		This is because lblock node.RawLBlock compiled
				(*lblock.Statements)[idx1][idx2] = models.LStatement{Operators: Statement.Operators, Exec: key}
			}(j, k, Statement)
		}
	}

	wg.Wait()
	return CompileLogic(&lblock)
}

func TrainData(parent *models.DialogNode, actorID string, nodes *[]*models.DialogNode, redisWriter chan common.RedisCommand, publishID string) {

	var compiledKey string
	if parent == nil {
		compiledKey = models.KeynavCompiledDialogRootWithinActor(publishID, actorID)
	} else {
		compiledKey = models.KeynavCompiledDialogNodeWithinActor(publishID, actorID, parent.ID.String())
	}

	dataset := snips.Dataset{}
	dataset.Language = snips.LanguageEnglish
	dataset.Entities = map[string]snips.Entity{}
	dataset.Intents = map[string]snips.Intent{}

	for _, node := range *nodes {

		logicalBlockLocation := models.KeynavCompiledEntity(publishID, models.AEIDDialogNode, node.ID.String())

		if node.UnknownHandler {
			// TODO: Handle unknown
			continue
		}

		intent := snips.Intent{}

		for _, input := range node.EntryInput {
			// TODO: Support slots and entities
			utterance := snips.Utterance{}
			utterance.AddChunk(snips.UtteranceChunk{Text: input.Prepared()})
			intent.AddUtterance(utterance)
		}

		dataset.Intents[logicalBlockLocation] = intent
	}

	preparedDataset, err := json.Marshal(dataset)
	if err != nil {
		fmt.Println("Error in TrainData", err)
		// TODO: Handle errors
	}

	rq, err := http.NewRequest("POST", fmt.Sprintf("http://vishnu:8080/v1/train"), bytes.NewReader(preparedDataset))
	if err != nil {
		fmt.Println("Error in TrainData", err)
		// TODO: Handle errors
	}
	rq.Header.Add("content-type", "application/json")
	client := http.Client{}
	resp, err := client.Do(rq)

	trainedData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error in TrainData", err)
		// TODO: Handle errors
	}

	redisWriter <- common.RedisSET(compiledKey, trainedData)
}

// DialogNode is a helper function to compile.Dialog
// It compiles the node logical blocks, action bundles therein,
// and its child nodes recursively.
func DialogNode(node models.DialogNode, redisWriter chan common.RedisCommand, processed *common.SyncMapUUID, publishID string) {
	processed.Mutex.Lock()
	if processed.Value == nil {
		processed.Value = map[uuid.UUID]bool{}
	}
	if processed.Value[node.ID] {
		processed.Mutex.Unlock()
		return
	}
	processed.Value[node.ID] = true
	processed.Mutex.Unlock()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(node models.DialogNode) {
		defer wg.Done()

		bslice := []byte{}

		// Boolean flag whether dialog continues or ends
		if node.ChildNodes == nil {
			bslice = append(bslice, 0)
		} else {
			bslice = append(bslice, 1)
		}

		// Save the compiled logical blocks and action bundles
		bslice = append(bslice, compileNodeHelper(node, redisWriter, publishID)...)
		compiledKey := models.KeynavCompiledEntity(publishID, models.AEIDDialogNode, node.ID.String())

		// Send it to be written to Redis
		redisWriter <- common.RedisSET(compiledKey, bslice)

	}(node)

	if node.ChildNodes == nil {
		wg.Wait()
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		TrainData(&node, node.ActorID.String(), node.ChildNodes, redisWriter, publishID)
	}()

	// For every child node, recurse this operation
	wg.Add(len(*node.ChildNodes))
	for _, child := range *node.ChildNodes {
		go func(node models.DialogNode) {
			defer wg.Done()
			DialogNode(node, redisWriter, processed, publishID)
		}(*child)
	}
	wg.Wait()
}

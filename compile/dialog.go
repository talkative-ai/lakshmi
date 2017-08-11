package compile

import (
	"fmt"

	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
	"github.com/artificial-universe-maker/lakshmi/prepare"
)

func CompileDialog(id int, dialog models.AumDialog) {
	ch := make(chan helpers.BSliceIndex)
	processedLBlocks := make([]models.LBlock, len(dialog.Nodes))
	for i, node := range dialog.Nodes {
		prepare.BundleActions(i, node.LogicalSet.AlwaysExec, ch)
	}
	for bundled := range ch {
		processedLBlocks[bundled.Index] = models.LBlock{}
		//	compiled:{pub_id}:entities:{Dialogs}:{dialog_id}:{action_set}
		redisKey := fmt.Sprintf("compiled:1:entities:1:%i:%i", 1, 1)
		processedLBlocks[bundled.Index].AlwaysExec = redisKey
	}
}

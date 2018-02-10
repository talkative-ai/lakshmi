package compile

import (
	"fmt"

	"github.com/talkative-ai/core/common"
	"github.com/talkative-ai/core/models"
	"github.com/talkative-ai/lakshmi/helpers"
	"github.com/talkative-ai/lakshmi/prepare"
)

func Trigger(redisWriter chan common.RedisCommand, items *[]models.ProjectTriggerItem) error {

	bundle := uint64(0)

	for _, item := range *items {
		bundle++
		lblock := models.LBlock{}

		key := models.KeynavCompiledTriggerActionBundle(item.ProjectID.String(), item.ZoneID.String(), uint64(item.TriggerType), bundle)
		bslice := prepare.BundleActions(item.RawLBlock.AlwaysExec)
		lblock.AlwaysExec = key
		redisWriter <- common.RedisSET(key, bslice)

		compiled := helpers.CompileLogic(&lblock)
		key = models.KeynavCompiledTriggersWithinZone(item.ProjectID.String(), item.ZoneID.String())
		redisWriter <- common.RedisHSET(key, fmt.Sprintf("%v", item.TriggerType), compiled)
	}

	return nil
}

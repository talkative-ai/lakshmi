package compile

import (
	"fmt"

	"github.com/artificial-universe-maker/core/common"
	"github.com/artificial-universe-maker/core/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
	"github.com/artificial-universe-maker/lakshmi/prepare"
)

func Trigger(redisWriter chan common.RedisCommand, items *[]models.ProjectTriggerItem) error {

	bundle := uint64(0)

	for _, item := range *items {
		bundle++
		lblock := models.LBlock{}

		key := models.KeynavCompiledTriggerActionBundle(item.ProjectID, item.TriggerID, bundle)
		bslice := prepare.BundleActions(item.RawLBlock.AlwaysExec)
		lblock.AlwaysExec = key
		redisWriter <- common.RedisSET(key, bslice)

		compiled := helpers.CompileLogic(&lblock)
		key = models.KeynavCompiledTriggersWithinZone(item.ProjectID, item.ZoneID)
		redisWriter <- common.RedisHSET(key, fmt.Sprintf("%v", item.TriggerType), compiled)
	}

	return nil
}

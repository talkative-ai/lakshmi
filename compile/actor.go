package compile

import (
	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/models"
)

func Actor(redisWriter chan common.RedisCommand, items *[]models.ProjectItem) error {
	zoneActorMap := map[uint64]map[uint64]bool{}
	var projectID uint64
	for _, item := range *items {
		if _, ok := zoneActorMap[item.ZoneID]; !ok {
			zoneActorMap[item.ZoneID] = map[uint64]bool{}
		}
		projectID = item.ProjectID
		zoneActorMap[item.ZoneID][item.ActorID] = true
	}

	for zoneID, mapping := range zoneActorMap {
		for actorID := range mapping {
			redisWriter <- common.RedisSADD(models.KeynavCompiledActorsWithinZone(projectID, zoneID), actorID)
		}
	}

	return nil
}

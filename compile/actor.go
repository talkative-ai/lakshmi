package compile

import (
	"github.com/talkative-ai/core/common"
	"github.com/talkative-ai/core/models"
	uuid "github.com/talkative-ai/go.uuid"
)

func Actor(redisWriter chan common.RedisCommand, items *[]models.ProjectItem) error {
	zoneActorMap := map[uuid.UUID]map[uuid.UUID]bool{}
	var projectID uuid.UUID
	for _, item := range *items {
		if _, ok := zoneActorMap[item.ZoneID]; !ok {
			zoneActorMap[item.ZoneID] = map[uuid.UUID]bool{}
		}
		projectID = item.ProjectID
		zoneActorMap[item.ZoneID][item.ActorID] = true
	}

	for zoneID, mapping := range zoneActorMap {
		for actorID := range mapping {
			redisWriter <- common.RedisSADD(models.KeynavCompiledActorsWithinZone(projectID.String(), zoneID.String()), actorID.String())
		}
	}

	return nil
}

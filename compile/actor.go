package compile

import (
	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/keynav"
)

func Actor(redisWriter chan common.RedisCommand, items *[]common.ProjectItem) error {
	zoneActorMap := map[uint64]map[uint64]bool{}
	var projectID uint64
	for _, item := range *items {
		projectID = item.ProjectID
		zoneActorMap[item.ZoneID][item.ActorID] = true
	}

	for zoneID, mapping := range zoneActorMap {
		for actorID := range mapping {
			redisWriter <- common.RedisSADD(keynav.CompiledActorsWithinZone(projectID, zoneID), actorID)
		}
	}

	return nil
}

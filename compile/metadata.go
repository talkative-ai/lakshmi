package compile

import (
	"fmt"
	"strings"

	"github.com/talkative-ai/core/common"
	"github.com/talkative-ai/core/models"
)

// Metadata saves all of the static and dynamic project metadata
func Metadata(redisWriter chan common.RedisCommand, project models.AumProject, items *[]models.ProjectItem, version int64) error {
	redisWriter <- common.RedisHSET(models.KeynavProjectMetadataStatic(project.ID.String()), "title", []byte(project.Title))
	redisWriter <- common.RedisHSET(models.KeynavProjectMetadataStatic(project.ID.String()), "start_zone_id", []byte(fmt.Sprintf("%v", project.StartZoneID.UUID.String())))
	redisWriter <- common.RedisHSET(models.KeynavProjectMetadataStatic(project.ID.String()), "pubver", []byte(fmt.Sprintf("%v", version)))
	redisWriter <- common.RedisHSET(models.KeynavGlobalMetaProjects(), strings.ToUpper(project.Title), []byte(fmt.Sprintf("%v", project.ID.String())))

	zoneIDs := map[string]bool{}
	for _, item := range *items {
		zoneIDs[fmt.Sprintf("%v", item.ZoneID)] = true
	}

	for id := range zoneIDs {
		redisWriter <- common.RedisSADD(fmt.Sprintf("%v:%v", models.KeynavProjectMetadataStatic(project.ID.String()), "all_zones"), []byte(id))
	}

	return nil
}

package compile

import (
	"fmt"
	"strings"

	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/keynav"
	"github.com/artificial-universe-maker/go-utilities/models"
)

// Metadata saves all of the static and dynamic project metadata
func Metadata(redisWriter chan common.RedisCommand, project models.AumProject) error {
	redisWriter <- common.RedisHSET(keynav.ProjectMetadataStatic(project.ID), "title", []byte(project.Title))
	redisWriter <- common.RedisHSET(keynav.ProjectMetadataStatic(project.ID), "start_zone_id", []byte(fmt.Sprintf("%v", project.StartZoneID.Int64)))
	redisWriter <- common.RedisHSET(keynav.GlobalMetaProjects(), strings.ToUpper(project.Title), []byte(fmt.Sprintf("%v", project.ID)))
	return nil
}

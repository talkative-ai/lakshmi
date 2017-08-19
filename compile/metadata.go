package compile

import (
	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/keynav"
	"github.com/artificial-universe-maker/go-utilities/models"
)

func CompileMetadata(redisWriter chan common.RedisCommand, project models.AumProject) error {
	redisWriter <- common.RedisSET(keynav.ProjectMetadataStaticProperty(project.ID, "title"), []byte(project.Title))
	return nil
}

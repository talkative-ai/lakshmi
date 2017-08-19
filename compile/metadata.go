package compile

import (
	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/keynav"
	"github.com/artificial-universe-maker/go-utilities/models"
)

func CompileMetadata(redisWriter chan common.RedisBytes, project models.AumProject) error {
	redisWriter <- common.RedisBytes{
		Key:   keynav.ProjectMetadataStaticProperty(project.ID, "title"),
		Bytes: []byte(project.Title),
	}

	return nil
}

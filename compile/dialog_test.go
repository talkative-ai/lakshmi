package compile

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
)

func TestCompileDialog(t *testing.T) {
	/**
	logicRaw := `{
		"always": 4000,
		"statements": [
			[{
				"conditions": [{
					"eq": {
						"123": "bar",
						"456": "world"
					},
					"gt": {
						"789": 100
					}
				}],
				"then": [
					1000
				]
			}, {
				"conditions": [{
					"eq": {
						"321": "foo",
						"654": "hello"
					},
					"lte": {
						"1231": 100
					}
				}],
				"then": [
					2000
				]
			}, {
				"then": [
					3000
				]
			}]
		]
	}`
	**/
	block := &models.RawLBlock{}
	block.AlwaysExec = models.AumActionSet{}
	block.AlwaysExec.PlaySounds = make([]models.ARAPlaySound, 2)
	block.AlwaysExec.PlaySounds[0].SoundType = models.ARAPlaySoundTypeText
	block.AlwaysExec.PlaySounds[0].Value = "Hello world"
	block.AlwaysExec.PlaySounds[1].SoundType = models.ARAPlaySoundTypeAudio
	block.AlwaysExec.PlaySounds[1].Value, _ = url.Parse("https://upload.wikimedia.org/wikipedia/commons/b/bb/Test_ogg_mp3_48kbps.wav")

	dialog := models.AumDialog{}
	dialog.Nodes = make([]models.AumDialogNode, 1)
	dialog.Nodes[0] = models.AumDialogNode{}
	dialog.Nodes[0].LogicalSet = *block

	redisWriter := make(chan helpers.RedisBytes)

	CompileDialog(dialog, redisWriter)

	fmt.Println(<-redisWriter)
	fmt.Println(<-redisWriter)
}

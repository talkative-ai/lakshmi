package compile

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/go-utilities/providers"
	"github.com/artificial-universe-maker/lakshmi/helpers"
)

func TestCompileDialog(t *testing.T) {
	block := &models.RawLBlock{}
	block.AlwaysExec = models.AumActionSet{}
	block.AlwaysExec.PlaySounds = make([]models.ARAPlaySound, 1)
	block.AlwaysExec.PlaySounds[0].SoundType = models.ARAPlaySoundTypeText
	block.AlwaysExec.PlaySounds[0].Value = "Hello world"
	stmt := make([][]models.RawLStatement, 1)
	block.Statements = &stmt
	(*block.Statements)[0] = make([]models.RawLStatement, 1)
	(*block.Statements)[0][0].Exec = models.AumActionSet{}
	(*block.Statements)[0][0].Exec.PlaySounds = make([]models.ARAPlaySound, 2)
	(*block.Statements)[0][0].Exec.PlaySounds[0].SoundType = models.ARAPlaySoundTypeText
	(*block.Statements)[0][0].Exec.PlaySounds[0].Value = "This is AUM!"
	(*block.Statements)[0][0].Exec.PlaySounds[1].SoundType = models.ARAPlaySoundTypeAudio
	(*block.Statements)[0][0].Exec.PlaySounds[1].Value, _ = url.Parse("https://upload.wikimedia.org/wikipedia/commons/b/bb/Test_ogg_mp3_48kbps.wav")

	dialog := models.AumDialogNode{}
	i := uint64(9001)
	dialog.ID = &i
	dialog.LogicalSet = *block
	dialog.EntryInput = append(dialog.EntryInput, models.AumDialogInputGreeting)
	dialog.EntryInput = append(dialog.EntryInput, models.AumDialogInputQuestionVerb)

	block2 := &models.RawLBlock{}
	block2.AlwaysExec = models.AumActionSet{}
	block2.AlwaysExec.PlaySounds = make([]models.ARAPlaySound, 1)
	block2.AlwaysExec.PlaySounds[0].SoundType = models.ARAPlaySoundTypeText
	block2.AlwaysExec.PlaySounds[0].Value = "I'm a nested dialog node. Goodbye"

	dialog2 := models.AumDialogNode{}
	i2 := uint64(9002)
	dialog2.ID = &i2
	dialog2.LogicalSet = *block2
	dialog2.EntryInput = append(dialog2.EntryInput, models.AumDialogInputFarewell)

	parents := make([]models.AumDialogNode, 1)
	parents[0] = dialog
	dialog2.ParentNodes = &parents

	children := make([]models.AumDialogNode, 1)
	children[0] = dialog2
	dialog.ChildNodes = &children

	redisWriter := make(chan helpers.RedisBytes)
	defer close(redisWriter)

	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("REDIS_PASSWORD", "")
	redis, err := providers.ConnectRedis()
	if err != nil {
		fmt.Println(err)
		return
	}

	defer redis.Close()

	go func() {
		for v := range redisWriter {
			redis.Set(v.Key, v.Bytes, 0)
		}
	}()

	CompileDialog(1, 0, dialog, redisWriter)
}

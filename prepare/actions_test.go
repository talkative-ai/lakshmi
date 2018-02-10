package prepare

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/talkative-ai/core/models"
)

func TestBundleActions(t *testing.T) {

	AAS := models.AumActionSet{}
	AAS.PlaySounds = make([]models.ARAPlaySound, 2)
	AAS.PlaySounds[0].SoundType = models.ARAPlaySoundTypeText
	AAS.PlaySounds[0].Val = "Hello world"
	AAS.PlaySounds[1].SoundType = models.ARAPlaySoundTypeAudio
	AAS.PlaySounds[1].Val, _ = url.Parse("https://upload.wikimedia.org/wikipedia/commons/b/bb/Test_ogg_mp3_48kbps.wav")

	b := BundleActions(AAS)
	fmt.Println(b)
}

package prepare

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/artificial-universe-maker/lakshmi/helpers"

	"github.com/artificial-universe-maker/go-utilities/models"
)

func TestBundleActions(t *testing.T) {

	AAS := models.AumActionSet{}
	AAS.PlaySounds = make([]models.ARAPlaySound, 2)
	AAS.PlaySounds[0].SoundType = models.ARAPlaySoundTypeText
	AAS.PlaySounds[0].Value = "Hello world"
	AAS.PlaySounds[1].SoundType = models.ARAPlaySoundTypeAudio
	AAS.PlaySounds[1].Value, _ = url.Parse("https://upload.wikimedia.org/wikipedia/commons/b/bb/Test_ogg_mp3_48kbps.wav")

	ch := make(chan helpers.BSliceIndex)

	go BundleActions(0, AAS, ch)

	for v := range ch {
		fmt.Println(v)
		close(ch)
	}
}

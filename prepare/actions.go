package prepare

import (
	"github.com/artificial-universe-maker/go-utilities/models"
	"github.com/artificial-universe-maker/lakshmi/helpers"
)

func BundleActions(id int, AAS models.AumActionSet, ch chan helpers.BSliceIndex) {
	bundle := helpers.BSliceIndex{Index: id, Bslice: []byte{}}
	bslices := make([][]byte, len(AAS.PlaySounds))
	cinner := make(chan helpers.BSliceIndex)
	for i, a := range AAS.PlaySounds {
		go func(idx int, a models.ARAPlaySound, cinner chan helpers.BSliceIndex) {
			bytes := a.Compile()
			finished := helpers.BSliceIndex{Index: idx, Bslice: bytes}
			cinner <- finished
		}(i, a, cinner)
	}
	c := 0
	for bslice := range cinner {
		bslices[bslice.Index] = bslice.Bslice
		c++
		if c == len(AAS.PlaySounds) {
			close(cinner)
		}
	}
	ch <- bundle
}

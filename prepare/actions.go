package prepare

import (
	"encoding/binary"

	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/models"
)

func BundleActions(AAS models.AumActionSet) []byte {
	bundle := []byte{}
	bslices := make([][]byte, len(AAS.PlaySounds))
	cinner := make(chan common.BSliceIndex)
	i := 0
	for a := range AAS.Iterable() {
		go func(idx int, a models.AumRuntimeAction, cinner chan common.BSliceIndex) {
			bytes := []byte{}
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(a.GetAAID()))
			bytes = append(bytes, b...)
			bytes = append(bytes, a.Compile()...)
			finished := common.BSliceIndex{Index: idx, Bslice: bytes}
			cinner <- finished
		}(i, a, cinner)
		i++
	}
	c := 0
	for bslice := range cinner {
		bslices[bslice.Index] = bslice.Bslice
		c++
		if c == len(AAS.PlaySounds) {
			close(cinner)
		}
	}
	for _, bslice := range bslices {
		bundle = append(bundle, bslice...)
	}
	return bundle
}

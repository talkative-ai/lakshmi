package prepare

import (
	"encoding/binary"

	"github.com/talkative-ai/core/common"
	"github.com/talkative-ai/core/models"
)

func BundleActions(AAS models.ActionSet) []byte {
	bundle := []byte{}
	cinner := make(chan common.BSliceIndex)
	actionCount := 0
	for range AAS.Iterable() {
		actionCount++
	}
	bslices := make([][]byte, actionCount)
	i := 0
	for a := range AAS.Iterable() {
		go func(idx int, a models.RequestAction, cinner chan common.BSliceIndex) {
			bytes := []byte{}

			// Store the RAID
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(a.GetRAID()))
			bytes = append(bytes, b...)

			// Store the length of the compiled data
			// This should never reach 4GB but having a buffer is always good
			compiled := a.Compile()
			b = make([]byte, 4)
			binary.LittleEndian.PutUint32(b, uint32(len(compiled)))
			bytes = append(bytes, b...)

			// Store compiled data
			bytes = append(bytes, compiled...)
			finished := common.BSliceIndex{Index: idx, Bslice: bytes}
			cinner <- finished
		}(i, a, cinner)
		i++
	}
	c := 0
	for bslice := range cinner {
		bslices[bslice.Index] = bslice.Bslice
		c++
		// TODO: Handle other actions here
		if c == actionCount {
			close(cinner)
		}
	}
	for _, bslice := range bslices {
		bundle = append(bundle, bslice...)
	}
	return bundle
}

package helpers

import (
	"encoding/binary"

	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/models"
)

func compileHelper(o *models.OrGroup) []byte {
	compiled := []byte{}
	OperatorStrIntMap := models.GenerateOperatorStrIntMap()

	for _, OrGroup := range *o {
		var availableStatements models.OperatorInt
		for c := range OrGroup {
			availableStatements |= OperatorStrIntMap[c]
		}
		compiled = append(compiled, byte(availableStatements))
		for _, group := range OrGroup {
			for vr, val := range group {
				b := make([]byte, 8)
				binary.LittleEndian.PutUint64(b, uint64(vr))
				compiled = append(compiled, b...)
				switch v := val.(type) {
				case string:
					compiled = append(compiled, uint8(0))
					b := make([]byte, 2)
					binary.LittleEndian.PutUint16(b, uint16(len(v)))
					compiled = append(compiled, b...)
					compiled = append(compiled, []byte(v)...)
					break
				case int:
					compiled = append(compiled, uint8(1))
					b := make([]byte, 4)
					binary.LittleEndian.PutUint32(b, uint32(v))
					compiled = append(compiled, b...)
					break
				}
			}
		}
	}

	return compiled
}

func compileStatement(stmt models.LStatement, idx int, cinner chan common.BSliceIndex) {
	bslice := []byte{}

	// Store the number of operators
	bslice = append(bslice, uint8(len(*stmt.Operators)))
	// And compile every operator
	// This process is really small and we're already deep in goroutines
	// So no need to make concurrent
	bslice = append(bslice, compileHelper(stmt.Operators)...)

	b := make([]byte, 2)

	// Exec is, of course an ActionBundle key
	// Just store the length of the key and the key
	binary.LittleEndian.PutUint16(b, uint16(len(stmt.Exec)))
	bslice = append(bslice, b...)
	bslice = append(bslice, []byte(stmt.Exec)...)

	// Sending back up the chain for concatentation to the whole compiled thing
	bsliceidx := common.BSliceIndex{
		Bslice: bslice,
		Index:  idx,
	}
	cinner <- bsliceidx
}

func compileStatements(statements []models.LStatement, idx int, c chan common.BSliceIndex) {
	bslice := []byte{}

	// Store the number of statements
	bslice = append(bslice, uint8(len(statements)))

	// Just as in CompileLogic, we compile each item internally here
	cinner := make(chan common.BSliceIndex)
	for idx, stmt := range statements {
		go compileStatement(stmt, idx, cinner)
	}

	newBytes := make([][]byte, len(statements))
	reg := 0
	for b := range cinner {
		// Sort the results as they come in
		newBytes[b.Index] = b.Bslice
		reg++
		if reg == len(statements) {
			close(cinner)
		}
	}

	for _, b := range newBytes {
		// Append the compiled LStatement to final result
		bslice = append(bslice, b...)
	}

	// We could store the length of the entire []LStatement here
	// That way when processing logic we could do it concurrently on Brahman
	// But logic is processed sequentially as the runtime state mutates

	bsliceidx := common.BSliceIndex{
		Bslice: bslice,
		Index:  idx,
	}
	c <- bsliceidx
}

// CompileLogic compiles the logical blocks within a dialog node or trigger
func CompileLogic(logic *models.LBlock) []byte {
	compiled := []byte{}

	b := make([]byte, 2)

	// Append the AlwaysExec length
	binary.LittleEndian.PutUint16(b, uint16(len(logic.AlwaysExec)))
	compiled = append(compiled, b...)
	// and the AlwaysExec key
	compiled = append(compiled, []byte(logic.AlwaysExec)...)

	if logic.Statements == nil {
		return compiled
	}

	// Save the number of []LStatement slices
	// Recall that logic.Statements.(type) == *[][]LStatement
	compiled = append(compiled, uint8(len(*logic.Statements)))

	// Prepare to compile the []LStatement slices concurrently
	c := make(chan common.BSliceIndex)
	for idx, conditional := range *logic.Statements {
		go compileStatements(conditional, idx, c)
	}

	// Used to organize the compiled values as they come in
	newBytes := make([][]byte, len(*logic.Statements))
	reg := 0
	for bslice := range c {
		// The channel passes back a byte slice (bslice) with the index
		// We sort by bslice index as they come in
		// Unsure if this is an anti-pattern or idiomatic. Just something I came up with.
		newBytes[bslice.Index] = bslice.Bslice
		reg++
		if reg == len(*logic.Statements) {
			close(c)
		}
	}

	// Finally iterate through the []LStatement bslices in order and append to the compiled output
	for _, bslice := range newBytes {
		compiled = append(compiled, bslice...)
	}

	return compiled
}

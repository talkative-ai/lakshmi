package helpers

import (
	"encoding/binary"

	"github.com/artificial-universe-maker/go-utilities/common"
	"github.com/artificial-universe-maker/go-utilities/models"
)

type lStatementsIndex struct {
	Statements []models.LStatement
	Index      int
}

type lStatementIndex struct {
	Stmt  models.LStatement
	Index int
}

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

func compileStatement(stmtidx *lStatementIndex, cinner chan common.BSliceIndex) {
	bslice := []byte{}

	if stmtidx.Stmt.Operators != nil {
		bslice = append(bslice, uint8(len(*stmtidx.Stmt.Operators)))
		bslice = append(bslice, compileHelper(stmtidx.Stmt.Operators)...)
	}

	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(len(stmtidx.Stmt.Exec)))
	bslice = append(bslice, b...)
	bslice = append(bslice, []byte(stmtidx.Stmt.Exec)...)
	bsliceidx := common.BSliceIndex{
		Bslice: bslice,
		Index:  stmtidx.Index,
	}
	cinner <- bsliceidx
}

func compileStatements(cidx *lStatementsIndex, c chan common.BSliceIndex) {
	bslice := []byte{}

	bslice = append(bslice, uint8(len(cidx.Statements)))

	cinner := make(chan common.BSliceIndex)
	for idx, stmt := range cidx.Statements {
		go compileStatement(&lStatementIndex{
			Stmt:  stmt,
			Index: idx,
		}, cinner)
	}

	newBytes := make([][]byte, len(cidx.Statements))

	reg := 0
	for b := range cinner {
		newBytes[b.Index] = b.Bslice
		reg++
		if reg == len(cidx.Statements) {
			close(cinner)
		}
	}

	for _, b := range newBytes {
		bslice = append(bslice, b...)
	}

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(len(bslice)))

	finished := []byte{}
	// Overall length of the statement byte array
	// Useful for jumping through statements
	finished = append(finished, b...)

	// Compiled statements
	finished = append(finished, bslice...)

	bsliceidx := common.BSliceIndex{
		Bslice: finished,
		Index:  cidx.Index,
	}
	c <- bsliceidx
}

// CompileLogic compiles the logical blocks within a dialog node or trigger
func CompileLogic(logic *models.LBlock) []byte {
	compiled := []byte{}

	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(len(logic.AlwaysExec)))
	compiled = append(compiled, b...)
	compiled = append(compiled, []byte(logic.AlwaysExec)...)
	if logic.Statements == nil {
		return compiled
	}

	compiled = append(compiled, uint8(len(*logic.Statements)))

	c := make(chan common.BSliceIndex)
	for idx, conditional := range *logic.Statements {
		go compileStatements(&lStatementsIndex{
			Statements: conditional,
			Index:      idx,
		}, c)
	}

	newBytes := make([][]byte, len(*logic.Statements))

	reg := 0
	for bslice := range c {
		newBytes[bslice.Index] = bslice.Bslice
		reg++
		if reg == len(*logic.Statements) {
			close(c)
		}
	}

	for _, bslice := range newBytes {
		compiled = append(compiled, bslice...)
	}

	return compiled
}

package helpers

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/artificial-universe-maker/core/models"
)

func TestCompileLogic(t *testing.T) {
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
	block := &models.LBlock{}
	json.Unmarshal([]byte(logicRaw), block)
	compiled := CompileLogic(block)
	fmt.Printf("%+v", compiled)
}

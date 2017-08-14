package main

import "testing"
import "fmt"

func TestCompiler(t *testing.T) {
	err := initiateCompiler(1)
	if err != nil {
		fmt.Println(err)
		t.Fail()
		return
	}
}

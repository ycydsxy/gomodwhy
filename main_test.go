package main

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestImportMathInTest(t *testing.T) {
	fmt.Println(sha256.New())
}

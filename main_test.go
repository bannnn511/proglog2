package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

var (
	enc      = binary.BigEndian
	lenWidth = 8
)

const fileTestPath = "./test.txt"

func TestMain(m *testing.M) {
	code := m.Run()

	_, err := os.Stat(fileTestPath)
	if err == nil {
		err = os.Remove(fileTestPath)
	}

	if err != nil {
		return
	}
	os.Exit(code)
}

func TestRandom(t *testing.T) {
	test := []byte("he")
	out := binary.BigEndian.Uint16(test)
	//uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
	fmt.Printf("%b\n", test)
	fmt.Println(test)
	fmt.Println(out)
}

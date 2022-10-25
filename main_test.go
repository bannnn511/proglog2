package main

import (
	"fmt"
	"os"
	"testing"
	"time"
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

type test struct {
	event chan int
}

func (t test) handle() {
	for x := range t.event {
		fmt.Println(x)
	}
}

func TestRandom(t *testing.T) {
	test := &test{event: make(chan int)}

	go test.handle()

	test.event <- 1
	time.Sleep(time.Minute * 1)
}

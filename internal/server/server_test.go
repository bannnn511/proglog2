package server

import (
	"fmt"
	"net"
	"os"
	"proglog/internal/log"
	"testing"
)

func TestSetup(t *testing.T) {
	lis, _ := net.Listen("tcp", "localhost:8080")

	dir, _ := os.MkdirTemp("", "log-test")
	defer os.Remove(dir)

	c := log.Config{}

	cLog, err := log.NewLog(dir, c)
	if err != nil {
		t.Errorf("log.NewLog() error %v", err)
	}
	config := Config{CommitLog: cLog}
	server, err := NewGrpcServer(&config)
	if err != nil {
		return
	}

	fmt.Println("GRPC serving...")
	err = server.Serve(lis)
	if err != nil {
		t.Errorf("grpc Server() error %v", err)
	}

}

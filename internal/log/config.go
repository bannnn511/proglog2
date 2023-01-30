package log

import "github.com/hashicorp/raft"

type Segment struct {
	MaxStoreBytes uint64
	MaxIndexBytes uint64
	InitialOffset uint64
}

type Config struct {
	Segment
	Raft struct {
		raft.Config
		BindAddr    string
		StreamLayer *StreamLayer
		Bootstrap   bool
	}
}

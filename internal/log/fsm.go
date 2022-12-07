package log

import (
	"bytes"
	"io"
	api "proglog/api/v1"

	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
)

type fsm struct {
	log *Log
}

// Apply is invoked after committing a new log entry.
func (f fsm) Apply(log *raft.Log) interface{} {
	buf := log.Data
	switch RequestType(buf[0]) {
	case AppendRequestType:
		return f.applyAppend(buf[1:])
	default:
	}

	return nil
}

// applyAppends unmarshal b into message then append the message into the log.
func (f fsm) applyAppend(b []byte) interface{} {
	var message api.ProduceRequest
	if err := proto.Unmarshal(b, &message); err != nil {
		return nil
	}

	offset, err := f.log.Append(message.Record)
	if err != nil {
		return nil
	}

	return &api.ProduceResponse{Offset: offset}
}

type snapshot struct {
	reader io.Reader
}

func (s snapshot) Release() {}

func (s snapshot) Persist(sink raft.SnapshotSink) error {
	_, err := io.Copy(sink, s.reader)
	if err != nil {
		return sink.Cancel()
	}
	return sink.Close()
}

var _ raft.FSMSnapshot = (*snapshot)(nil)

// Snapshot returns an FSMSnapshot that represents a point-in-time snapshot
// of the fsm' s state.
func (f fsm) Snapshot() (raft.FSMSnapshot, error) {
	reader := f.log.Reader()

	return &snapshot{reader: reader}, nil
}

// Restore restores an FSM from a snapshot.
func (f fsm) Restore(r io.ReadCloser) error {
	b := make([]byte, lenWidth)
	buf := bytes.Buffer{}

	for i := 0; ; i++ {
		_, err := io.ReadFull(r, b)
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		var record api.Record
		if err := proto.Unmarshal(b, &record); err != nil {
			return err
		}

		if i == 0 {
			f.log.config.InitialOffset = record.Offset
		} else {
			if err := f.log.Reset(); err != nil {
				return err
			}
		}

		_, err = f.log.Append(&record)
		if err != nil {
			return err
		}

		buf.Reset()
	}

	return nil
}

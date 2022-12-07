package log

import (
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"path/filepath"
	api "proglog/api/v1"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"google.golang.org/protobuf/proto"
)

// DistributedLog is a distributed Log with raft for handling consensus.
type DistributedLog struct {
	config Config
	Log    *Log
	raft   *raft.Raft
}

// NewDistributedLog creates a new distributed log.
// It will create a new log and raft instance.
func NewDistributedLog(dataDir string, config Config) (*DistributedLog, error) {
	d := &DistributedLog{
		config: config,
	}
	err := d.setupLog(dataDir)
	if err != nil {
		return nil, err
	}
	err = d.setupRaft(dataDir)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// setupLog creates the log for this server, where the server will store user's records.
func (d *DistributedLog) setupLog(dataDir string) error {
	directoryPath := dataDir + "/log"
	err := os.MkdirAll(directoryPath, 0755)
	if err != nil {
		return err
	}

	d.Log, err = NewLog(directoryPath, d.config)
	if err != nil {
		return err
	}

	return nil
}

func (d *DistributedLog) setupRaft(dataDir string) error {
	// FSN that applies the command given to Raft.
	fsm := &fsm{}

	// START: log store where Raft will store commands.
	logDirectory := filepath.Join(dataDir, "raft", "logs")
	if err := os.MkdirAll(logDirectory, 0755); err != nil {
		return err
	}

	logConfig := d.config
	logConfig.Segment.InitialOffset = 1
	logStore, err := NewLog(logDirectory, logConfig)
	if err != nil {
		return err
	}
	// END: log store where Raft will store commands.

	// START: stable store where Raft will store metadata.
	stableDirectory := filepath.Join(dataDir, "raft", "stable")
	if err = os.MkdirAll(stableDirectory, 0755); err != nil {
		return err
	}

	boltStore, err := raftboltdb.New(raftboltdb.Options{
		Path: filepath.Join(stableDirectory),
	})
	if err != nil {
		return err
	}
	// END: stable store

	// START: raft SnapshotStore
	var retain = 1
	snapshotStore, err := raft.NewFileSnapshotStore(
		filepath.Join(dataDir, "raft"),
		retain,
		os.Stderr,
	)
	if err != nil {
		return err
	}
	// END: raft SnapshotStore

	// START: raft transport
	maxPool := 5
	timeout := 10 * time.Second
	transport := raft.NewNetworkTransport(
		d.config.Raft.StreamLayer,
		maxPool,
		timeout,
		os.Stderr,
	)

	// END: raft transport

	// START: raft configuration
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = d.config.Raft.LocalID
	if d.config.Raft.HeartbeatTimeout != 0 {
		raftConfig.HeartbeatTimeout = d.config.Raft.HeartbeatTimeout
	}

	if d.config.Raft.ElectionTimeout != 0 {
		raftConfig.ElectionTimeout = d.config.Raft.ElectionTimeout
	}

	if d.config.Raft.LeaderLeaseTimeout != 0 {
		raftConfig.LeaderLeaseTimeout = d.config.Raft.LeaderLeaseTimeout
	}

	if d.config.Raft.CommitTimeout != 0 {
		raftConfig.CommitTimeout = d.config.Raft.CommitTimeout
	}
	// END: raft configuration

	// START: raft
	d.raft, err = raft.NewRaft(raftConfig, fsm, logStore, boltStore, snapshotStore, transport)
	if err != nil {
		return err
	}

	hasState, err := raft.HasExistingState(logStore, boltStore, snapshotStore)
	if err != nil {
		return err
	}

	// if raft has no state, bootstrap it
	// first server is usually bootstrap itself as the only voter.
	// leader will add more servers into the cluster.
	// subsequent servers don't bootstrap themselves.
	if !hasState {
		bootstrapConfig := raft.Configuration{
			Servers: []raft.Server{
				{
					ID: d.config.Raft.LocalID,
				},
			},
		}
		err := raft.BootstrapCluster(raftConfig, logStore, boltStore, snapshotStore, transport, bootstrapConfig)
		if err != nil {
			return err
		}
	}
	// END: raft

	return nil
}

type RequestType uint8

const (
	AppendRequestType RequestType = iota
)

func (d *DistributedLog) Append(record *api.Record) (uint64, error) {
	response, err := d.apply(AppendRequestType, record)
	if err != nil {
		return 0, err
	}

	produceResponse, ok := response.(*api.ProduceResponse)
	if !ok {
		return 0, errors.New("response not type ProduceResponse")
	}

	return produceResponse.Offset, nil
}

// apply wraps Raft's api to apply request and return response.
func (d *DistributedLog) apply(requestType RequestType, message proto.Message) (interface{}, error) {
	var buf bytes.Buffer
	buf.Write([]byte{byte(requestType)})
	marshal, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	buf.Write(marshal)

	timeout := 10 * time.Second
	applyFuture := d.raft.Apply(buf.Bytes(), timeout)
	if applyFuture.Error() != nil {
		return nil, err
	}

	return applyFuture.Response(), nil
}

// Reade is eventually consistent read.
func (d *DistributedLog) Read(offset uint64) (*api.Record, error) {
	return d.Log.Read(offset)
}

type StreamLayer struct {
	ln              net.Listener
	serverTLSConfig *tls.Config
	peerTLSConfig   *tls.Config
}

func (s StreamLayer) Accept() (net.Conn, error) {
	//TODO implement me
	panic("implement me")
}

func (s StreamLayer) Close() error {
	//TODO implement me
	panic("implement me")
}

func (s StreamLayer) Addr() net.Addr {
	//TODO implement me
	panic("implement me")
}

func (s StreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	//TODO implement me
	panic("implement me")
}

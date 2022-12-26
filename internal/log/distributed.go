package log

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	api "proglog/api/v1"
	"proglog/internal/discovery"
	"time"

	"go.uber.org/zap"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"google.golang.org/protobuf/proto"
)

const DebugMode = 1

// DistributedLog is a distributed Log with raft for handling consensus.
type DistributedLog struct {
	config Config
	log    *Log
	raft   *raft.Raft
	logger *zap.SugaredLogger
}

var _ discovery.Handler = (*DistributedLog)(nil)

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

	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	d.logger = logger.Sugar()

	return d, nil
}

// setupLog creates the log for this server, where the server will store user's records.
func (d *DistributedLog) setupLog(dataDir string) error {
	directoryPath := filepath.Join(dataDir, "log")
	if err := os.MkdirAll(directoryPath, 0755); err != nil {
		return err
	}

	var err error
	d.log, err = NewLog(directoryPath, d.config)

	return err
}

func (d *DistributedLog) setupRaft(dataDir string) error {
	// FSN that applies the command given to Raft.
	fsm := &fsm{
		log: d.log,
	}

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
	boltStore, err := raftboltdb.New(raftboltdb.Options{
		Path: filepath.Join(dataDir, "raft", "stable"),
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

	if d.config.Raft.Bootstrap {
		config := raft.Configuration{
			Servers: []raft.Server{{
				ID:      raftConfig.LocalID,
				Address: transport.LocalAddr(),
			}},
		}
		err = d.raft.BootstrapCluster(config).Error()
	}
	// END: raft

	return err
}

// GetServers returns a list of raft's servers.
func (d *DistributedLog) GetServers() ([]*api.Server, error) {
	raftConfigurations := d.raft.GetConfiguration()
	if err := raftConfigurations.Error(); err != nil {
		return nil, err
	}

	var servers []*api.Server
	for _, server := range raftConfigurations.Configuration().Servers {
		leaderAddress, leaderId := d.raft.LeaderWithID()
		servers = append(servers, &api.Server{
			Id:       string(server.ID),
			RpcAddr:  string(server.Address),
			IsLeader: server.ID == leaderId && server.Address == leaderAddress,
		})
	}

	return servers, nil
}

type RequestType uint8

const (
	AppendRequestType RequestType = iota
)

func (d *DistributedLog) Append(record *api.Record) (uint64, error) {
	d.slog("Append", "value", record.Value, "offset", record.Offset)
	response, err := d.apply(AppendRequestType, &api.ProduceRequest{Record: record})
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
	_, err := buf.Write([]byte{byte(requestType)})
	if err != nil {
		return nil, err
	}

	marshal, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}

	_, err = buf.Write(marshal)
	if err != nil {
		return nil, err
	}

	timeout := 10 * time.Second
	applyFuture := d.raft.Apply(buf.Bytes(), timeout)
	if err := applyFuture.Error(); err != nil {
		d.error("failed to apply request", "err", err)
		return nil, err
	}

	return applyFuture.Response(), nil
}

// Reade is eventually consistent read.
func (d *DistributedLog) Read(offset uint64) (*api.Record, error) {
	return d.log.Read(offset)
}

// START: Service Discovery handler

func (d *DistributedLog) Join(id, addr string) error {
	configFuture := d.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}

	serverID := raft.ServerID(id)
	serverAddr := raft.ServerAddress(addr)

	for _, server := range configFuture.Configuration().Servers {
		if server.ID == serverID || server.Address == serverAddr {
			if server.ID == serverID && server.Address == serverAddr {
				return nil
			}

			removeFuture := d.raft.RemoveServer(serverID, 0, 0)
			if err := removeFuture.Error(); err != nil {
				return err
			}
		}

		addFuture := d.raft.AddVoter(serverID, serverAddr, 0, 0)
		if err := addFuture.Error(); err != nil {
			return err
		}
	}

	return nil
}

func (d *DistributedLog) Leave(id string) error {
	d.slog("Leave", "id", id)
	indexFuture := d.raft.RemoveServer(raft.ServerID(id), 0, 0)
	return indexFuture.Error()
}

func (d *DistributedLog) WaitForLeader(timeout time.Duration) error {
	timeoutC := time.After(timeout)
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-timeoutC:
			err := errors.New("timeout waiting for leader")
			d.error(err.Error())

			return err
		case <-ticker.C:
			addr, _ := d.raft.LeaderWithID()
			if addr != "" {
				return nil
			}
		}
	}
}

func (d *DistributedLog) Close() error {
	d.slog("Close")
	if err := d.raft.Shutdown().Error(); err != nil {
		fmt.Println("error shutting down raft", err)
		return err
	}
	return d.log.Close()
}

// slog logs a debugging message is DebugCM > 0.
func (d *DistributedLog) slog(format string, args ...interface{}) {
	if DebugMode > 0 {
		format = fmt.Sprintf("[%v] ", d.config.Raft.LocalID) + format
		d.logger.Infow(format, args...)
	}
}

// slog logs a debugging message is DebugCM > 0.
func (d *DistributedLog) error(format string, args ...interface{}) {
	if DebugMode > 0 {
		format = fmt.Sprintf("[%v] ", d.config.Raft.LocalID) + format
		d.logger.Errorf(format, args...)
	}
}

// END: Service Discovery handler

var _ raft.StreamLayer = (*StreamLayer)(nil)

type StreamLayer struct {
	ln              net.Listener
	serverTLSConfig *tls.Config
	peerTLSConfig   *tls.Config
}

func NewStreamLayer(ln net.Listener, serverTLSConfig, peerTLSConfig *tls.Config) *StreamLayer {
	return &StreamLayer{
		ln:              ln,
		serverTLSConfig: serverTLSConfig,
		peerTLSConfig:   peerTLSConfig,
	}
}

const RaftRPC = 1

func (s StreamLayer) Accept() (net.Conn, error) {
	conn, err := s.ln.Accept()
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 1)
	if _, err := conn.Read(buffer); err != nil {
		return nil, err
	}

	if !bytes.Equal(buffer, []byte{byte(RaftRPC)}) {
		return nil, errors.New("not raft rpc")
	}

	if s.serverTLSConfig != nil {
		conn = tls.Server(conn, s.serverTLSConfig)
	}

	return conn, nil
}

func (s StreamLayer) Close() error {
	return s.ln.Close()
}

func (s StreamLayer) Addr() net.Addr {
	return s.ln.Addr()
}

// Dial makes out-going connections to other severs.
// When we connect, we will write RaftRPC byte to identify the connection type,
// so that we can multiplex Raft on the same port as our Log gRPC request.
func (s StreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout: timeout,
	}

	var conn, err = dialer.Dial("tcp", string(address))
	if err != nil {
		return nil, err
	}

	_, err = conn.Write([]byte{byte(RaftRPC)})
	if err != nil {
		return nil, err
	}

	if s.peerTLSConfig != nil {
		conn = tls.Client(conn, s.peerTLSConfig)
	}

	return conn, nil
}

package agent

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"proglog/internal/discovery"
	"proglog/internal/log"
	"proglog/internal/server"
	"sync"
	"time"

	"google.golang.org/grpc/credentials"

	"github.com/hashicorp/raft"

	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Agent struct {
	Config

	mux        cmux.CMux
	Log        *log.DistributedLog
	Server     *grpc.Server
	Membership *discovery.Membership

	shutdownLock sync.Mutex
	isShutdown   bool
}

type Config struct {
	ServerTLSConfig *tls.Config
	PeerTLSConfig   *tls.Config
	// DataDir stores the log and raft fata.
	DataDir string
	// Raft server id.
	NodeName string
	// BindAddr is the address that serf run on.
	BindAddr string
	// Port for client and raft connections.
	RpcPort            int
	StartJoinAddresses []string
	Boostrap           bool
}

func New(config Config) (*Agent, error) {
	agent := &Agent{
		Config: config,
	}

	setups := []func() error{
		agent.setupLogger,
		agent.setupMux,
		agent.setupLog,
		agent.setupServer,
		agent.setupMembership,
	}

	for _, setup := range setups {
		err := setup()
		if err != nil {
			return nil, err
		}
	}
	go func() {
		err := agent.serve()
		if err != nil {
			panic(err)
		}
	}()

	return agent, nil
}

func (c Config) RPCAddr() (string, error) {
	host, _, err := net.SplitHostPort(c.BindAddr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", host, c.RpcPort), nil
}

// setupLogger setups zap logger.
func (a *Agent) setupLogger() error {
	logger := zap.L()
	zap.ReplaceGlobals(logger.Named("Agent"))

	return nil
}

func (a *Agent) setupLog() error {
	raftLn := a.mux.Match(func(reader io.Reader) bool {
		b := make([]byte, 1)
		if _, err := reader.Read(b); err != nil {
			return false
		}

		return bytes.Equal(b, []byte{byte(log.RaftRPC)})
	})

	raftConfig := log.Config{}
	raftConfig.Raft.StreamLayer = log.NewStreamLayer(raftLn, a.ServerTLSConfig, a.PeerTLSConfig)
	raftConfig.Raft.LocalID = raft.ServerID(a.NodeName)
	raftConfig.Raft.Bootstrap = a.Boostrap

	var err error
	a.Log, err = log.NewDistributedLog(a.DataDir, raftConfig)
	if err != nil {
		return err
	}

	if a.Boostrap {
		return a.Log.WaitForLeader(3 * time.Second)
	}

	return nil
}

func (a *Agent) setupServer() error {
	config := &server.Config{
		CommitLog:  a.Log,
		GetServers: a.Log,
	}

	var opts []grpc.ServerOption
	if a.ServerTLSConfig != nil {
		cred := credentials.NewTLS(a.ServerTLSConfig)
		opts = append(opts, grpc.Creds(cred))
	}

	var err error
	a.Server, err = server.NewGrpcServer(config, opts...)
	if err != nil {
		return err
	}

	grpcLn := a.mux.Match(cmux.Any())
	go func() {
		err := a.Server.Serve(grpcLn)
		if err != nil {
			_ = a.shutdown()
		}
	}()

	return err
}

func (a *Agent) setupMux() error {
	rpcAddr := fmt.Sprintf(":%d", a.RpcPort)
	listen, err := net.Listen("tcp", rpcAddr)
	if err != nil {
		return err
	}
	a.mux = cmux.New(listen)

	return nil
}

func (a *Agent) setupMembership() error {
	rpcAddr, err := a.Config.RPCAddr()
	if err != nil {
		return err
	}

	config := discovery.Config{
		NodeName:           a.NodeName,
		BindAddr:           a.BindAddr,
		StartJoinAddresses: a.StartJoinAddresses,
		Tags: map[string]string{
			"rpc_addr": rpcAddr,
		},
	}

	membership, err := discovery.New(
		a.Log,
		config,
	)

	if err != nil {
		return err
	}
	a.Membership = membership

	return nil
}

func (a *Agent) serve() error {
	if err := a.mux.Serve(); err != nil {
		_ = a.shutdown()

		return err
	}

	return nil
}

func (a *Agent) shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.isShutdown {
		return nil
	}
	a.isShutdown = true

	shutdownFunctions := []func() error{
		a.Membership.Leave,
		func() error {
			a.Server.GracefulStop()
			return nil
		},
		a.Log.Close,
	}

	for _, shutdownFunc := range shutdownFunctions {
		err := shutdownFunc()
		if err != nil {
			return err
		}
	}

	return nil
}

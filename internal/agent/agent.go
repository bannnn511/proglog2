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
	log        *log.DistributedLog
	server     *grpc.Server
	Membership *discovery.Membership

	shutdownLock sync.Mutex
	isShutdown   bool
	shutdowns    chan struct{}
}

type Config struct {
	ServerTLSConfig *tls.Config
	PeerTLSConfig   *tls.Config
	ACLModelFile    string
	ACLPolicyFile   string

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
		Config:    config,
		shutdowns: make(chan struct{}),
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
			fmt.Println(err)
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

	rpcAddr, err := a.Config.RPCAddr()
	if err != nil {
		return err
	}
	raftConfig.Raft.BindAddr = rpcAddr
	raftConfig.Raft.LocalID = raft.ServerID(a.NodeName)
	raftConfig.Raft.Bootstrap = a.Boostrap

	a.log, err = log.NewDistributedLog(a.DataDir, raftConfig)
	if err != nil {
		return err
	}

	if a.Boostrap {
		return a.log.WaitForLeader(3 * time.Second)
	}

	return nil
}

func (a *Agent) setupServer() error {
	config := &server.Config{
		CommitLog:   a.log,
		GetServerer: a.log,
	}

	var opts []grpc.ServerOption
	if a.ServerTLSConfig != nil {
		cred := credentials.NewTLS(a.ServerTLSConfig)
		opts = append(opts, grpc.Creds(cred))
	}

	var err error
	a.server, err = server.NewGrpcServer(config, opts...)
	if err != nil {
		return err
	}

	grpcLn := a.mux.Match(cmux.Any())
	go func() {
		err := a.server.Serve(grpcLn)
		if err != nil {
			_ = a.Shutdown()
		}
	}()

	return err
}

func (a *Agent) setupMux() error {
	add, err := net.ResolveTCPAddr("tcp", a.BindAddr)
	if err != nil {
		return err
	}

	rpcAddr := fmt.Sprintf("%s:%d", add.IP.String(), a.RpcPort)
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
		a.log,
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
		fmt.Println("-------------- mux shutdown", err.Error())
		_ = a.Shutdown()

		return err
	}

	return nil
}

func (a *Agent) Shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.isShutdown {
		return nil
	}
	a.isShutdown = true
	close(a.shutdowns)

	shutdownFunctions := []func() error{
		a.Membership.Leave,
		func() error {
			a.server.GracefulStop()
			return nil
		},
		a.log.Close,
	}

	for _, shutdownFunc := range shutdownFunctions {
		if err := shutdownFunc(); err != nil {
			return err
		}
	}

	return nil
}

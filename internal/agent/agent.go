package agent

import (
	"fmt"
	"net"
	"proglog/internal/discovery"
	"proglog/internal/log"
	"proglog/internal/server"
	"sync"

	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Agent struct {
	Config

	mux          cmux.CMux
	Log          *log.DistributedLog
	Server       *grpc.Server
	Membership   *discovery.Membership
	shutdownLock sync.Mutex
	isShutdown   bool
}

type Config struct {
	DataDir            string
	NodeName           string
	BindAddr           string
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
	logStore, err := log.NewDistributedLog(a.DataDir, log.Config{})
	if err != nil {
		return err
	}

	a.Log = logStore

	return nil
}

func (a *Agent) setupServer() error {
	config := &server.Config{
		CommitLog:  a.Log,
		GetServers: a.Log,
	}

	serverPort, err := a.RPCAddr()
	if err != nil {
		return err
	}

	l, err := net.Listen("tcp", serverPort)
	if err != nil {
		return err
	}

	grpcServer, err := server.NewGrpcServer(config)
	if err != nil {
		return err
	}

	go func() {
		err := grpcServer.Serve(l)
		if err != nil {
			_ = a.shutdown()
		}
	}()
	a.Server = grpcServer

	return nil
}

func (a *Agent) setupMux() error {
	rpcAddr := fmt.Sprintf(":%d", a.RpcPort)
	listen, err := net.Listen("tcp", string(rpcAddr))
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

package agent

import (
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net"
	"proglog/internal/discovery"
	"proglog/internal/log"
	"proglog/internal/server"
	"sync"
)

type Agent struct {
	Config
	Log          *log.Log
	Server       *grpc.Server
	Membership   *discovery.Membership
	shutdownLock sync.Mutex
}

type Config struct {
	DataDir            string
	NodeName           string
	BindAddr           string
	StartJoinAddresses []string
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

	return agent, nil
}

// setupLogger setups zap logger.
func (a *Agent) setupLogger() error {
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(logger.Named("Agent"))

	return nil
}

func (a *Agent) setupLog() error {
	logStore, err := log.NewLog(a.DataDir, log.Config{})
	if err != nil {
		return err
	}

	a.Log = logStore

	return nil
}

func (a *Agent) setupServer() error {
	config := &server.Config{
		CommitLog: a.Log,
	}

	l, err := net.Listen("tcp", a.BindAddr)
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

func (a *Agent) setupMembership() error {
	config := discovery.Config{
		NodeName:           a.NodeName,
		BindAddr:           a.BindAddr,
		StartJoinAddresses: a.StartJoinAddresses,
	}

	membership, err := discovery.New(&log.Replicator{}, config)
	if err != nil {
		return err
	}
	a.Membership = membership

	return nil
}

func (a *Agent) shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	shutdownFuncs := []func() error{
		a.Membership.Leave,
		a.Log.Close,
		func() error {
			fmt.Println("agent here", a.Server)
			a.Server.GracefulStop()
			return nil
		},
	}

	for _, shutdownFunc := range shutdownFuncs {
		err := shutdownFunc()
		if err != nil {
			return err
		}
	}

	return nil
}

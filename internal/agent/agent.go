package agent

import (
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	v1 "proglog/api/v1"
	"proglog/internal/discovery"
	"proglog/internal/log"
	"proglog/internal/server"
	"sync"
)

type Agent struct {
	Config
	replicator   *log.Replicator
	Log          *log.Log
	Server       *grpc.Server
	Membership   *discovery.Membership
	shutdownLock sync.Mutex
}

type Config struct {
	DataDir            string
	NodeName           string
	BindAddr           string
	RpcPort            int
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

	rpcAddr, _ := agent.RPCAddr()
	fmt.Printf("Agent config %v, bindAddr %v, rpcPort %v\n", agent.BindAddr, agent.RpcPort, rpcAddr)

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
	//if err != nil {
	//	return err
	//}
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

	clientOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	target, err := a.RPCAddr()
	if err != nil {
		return err
	}

	cc, err := grpc.Dial(target, clientOptions...)
	if err != nil {
		return err
	}

	localServer := v1.NewLogClient(cc)
	if err != nil {
		return err
	}

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	a.replicator = &log.Replicator{
		LocalServer: localServer,
		DialOptions: opts,
	}
	membership, err := discovery.New(
		a.replicator,
		config,
	)
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
		a.replicator.Close,
		func() error {
			a.Server.GracefulStop()
			return nil
		},
		a.Log.Close,
	}

	for _, shutdownFunc := range shutdownFuncs {
		err := shutdownFunc()
		if err != nil {
			return err
		}
	}

	return nil
}

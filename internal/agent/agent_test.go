package agent

import (
	"context"
	"fmt"
	"os"
	api "proglog/api/v1"
	"proglog/util"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestAgent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	logger.Sugar()
	var agents []*Agent

	// START: Setup Agent
	for i := 0; i < 2; i++ {
		dataDir, err := os.MkdirTemp("/Users/bannnnn./Documents/test-log", fmt.Sprintf("agent-#%v-", i))
		require.NoError(t, err, "error create temp dir for agent")

		port, err := util.GetFreePort()
		require.NoError(t, err, "getFreePort")
		addr := fmt.Sprintf("%s:%d", "127.0.0.1", port)

		rpcPort, err := util.GetFreePort()
		require.NoError(t, err, "getFreePort")

		var startJoinAddresses []string
		if i != 0 {
			startJoinAddresses = append(startJoinAddresses, agents[0].BindAddr)
		}

		config := Config{
			DataDir:            dataDir,
			NodeName:           fmt.Sprintf("Node No.%v", i),
			BindAddr:           addr,
			RpcPort:            rpcPort,
			StartJoinAddresses: startJoinAddresses,
		}
		logger.Info("AGENT INFO-----",
			zap.String("node name", config.NodeName),
			zap.String("bind addr", config.BindAddr),
			zap.Int("rpc port", rpcPort),
		)

		agent, err := New(config)
		require.NoError(t, err, "create agent error")
		agents = append(agents, agent)
	}
	// END: Setup Agent

	// START: Handle shutting down agents
	defer func() {
		for _, agent := range agents {
			err := agent.shutdown()
			require.NoError(t, err, "agent shutdown error")
			require.NoError(t, os.RemoveAll(agent.DataDir))
		}
	}()
	// END: Handle shutting down agents

	//time.Sleep(1 * time.Second)
	// START: Leader log client
	leaderClient, err := client(agents[0])
	require.NoError(t, err, "error create leader client")

	message := "hello"
	leaderResponse, err := leaderClient.Produce(
		context.Background(),
		&api.ProduceRequest{
			Record: &api.Record{
				Value: []byte(message),
			},
		},
	)
	require.NoError(t, err, "leader produce error")

	consume, err := leaderClient.Consume(
		context.Background(),
		&api.ConsumeRequest{Offset: leaderResponse.Offset},
	)
	require.NoError(t, err, "leader client consume error")
	require.Equal(t, string(consume.Record.Value), message, "produce and consume must have equal value")
	// END: Leader log client

	// START: Test consumer
	consumerClient, err := client(agents[1])
	require.NoError(t, err, "error create client 1")

	//wait for replication
	//time.Sleep(3 * time.Second)
	response, err := consumerClient.Consume(
		context.Background(),
		&api.ConsumeRequest{
			Offset: leaderResponse.Offset,
		},
	)
	require.NoError(t, err, "consumer consume error")
	require.Equal(t, string(response.Record.Value), message)
}

func client(agent *Agent) (api.LogClient, error) {
	clientOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	target, err := agent.RPCAddr()
	if err != nil {
		return nil, err
	}

	cc, err := grpc.Dial(target, clientOptions...)

	if err != nil {
		return nil, err
	}
	logClient := api.NewLogClient(cc)

	return logClient, nil
}

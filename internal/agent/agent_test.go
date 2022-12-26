package agent_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	api "proglog/api/v1"
	"proglog/internal/agent"
	"proglog/internal/config"
	"proglog/internal/loadbalance"
	"proglog/util"
	"testing"
	"time"

	"google.golang.org/grpc/credentials"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestAgent(t *testing.T) {
	serverTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.ServerCertFile,
		KeyFile:       config.ServerKeyFile,
		CAFile:        config.CAFile,
		Server:        true,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	peerTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.RootClientCertFile,
		KeyFile:       config.RootClientKeyFile,
		CAFile:        config.CAFile,
		Server:        false,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	var agents []*agent.Agent

	// START: Setup Agent
	for i := 0; i < 2; i++ {
		dataDir, err := os.MkdirTemp("", fmt.Sprintf("agent-#%v-", i))
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

		agent, err := agent.New(agent.Config{
			DataDir:            dataDir,
			Boostrap:           i == 0,
			NodeName:           fmt.Sprintf("Node No.%v", i),
			BindAddr:           addr,
			RpcPort:            rpcPort,
			StartJoinAddresses: startJoinAddresses,
			PeerTLSConfig:      peerTLSConfig,
			ServerTLSConfig:    serverTLSConfig,
			ACLModelFile:       config.ACLModelFile,
			ACLPolicyFile:      config.ACLPolicyFile,
		},
		)
		require.NoError(t, err, "create agent error")
		agents = append(agents, agent)
	}
	// END: Setup Agent

	defer func() {
		for _, agent := range agents {
			err := agent.Shutdown()
			require.NoError(t, err, "agent shutdown error")
			require.NoError(t, os.RemoveAll(agent.DataDir))
		}
	}()

	time.Sleep(3 * time.Second)

	// START: Leader log client
	leaderClient := client(t, agents[0], peerTLSConfig)

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

	time.Sleep(3 * time.Second)

	consume, err := leaderClient.Consume(
		context.Background(),
		&api.ConsumeRequest{Offset: leaderResponse.Offset},
	)
	require.NoError(t, err, "leader client consume error")
	require.Equal(t, string(consume.Record.Value), message, "produce and consume must have equal value")
	// END: Leader log client

	// START: Test consumer
	consumerClient := client(t, agents[1], peerTLSConfig)
	response, err := consumerClient.Consume(
		context.Background(),
		&api.ConsumeRequest{
			Offset: leaderResponse.Offset,
		},
	)
	require.NoError(t, err, "consumer consume error")
	require.Equal(t, string(response.Record.Value), message)

}

func client(t *testing.T, agent *agent.Agent, tlsConfig *tls.Config) api.LogClient {
	tlsCreds := credentials.NewTLS(tlsConfig)
	clientOptions := []grpc.DialOption{grpc.WithTransportCredentials(tlsCreds)}
	target, err := agent.RPCAddr()
	require.NoError(t, err)

	cc, err := grpc.Dial(fmt.Sprintf("%s://%s",
		loadbalance.Name,
		target), clientOptions...)

	require.NoError(t, err)

	return api.NewLogClient(cc)
}

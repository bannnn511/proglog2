package agent

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	api "proglog/api/v1"
	"proglog/util"
	"testing"
)

func TestAgent(t *testing.T) {
	var agents []*Agent
	for i := 0; i < 3; i++ {
		dataDir, err := os.MkdirTemp("", "agent-test-log")
		require.NoError(t, err, "error create temp dir for agent")

		port, err := util.GetFreePort()
		require.NoError(t, err, "getFreePort")
		addr := fmt.Sprintf("%s:%d", "127.0.0.1", port)

		var startJoinAddresses []string
		if i != 0 {
			startJoinAddresses = append(startJoinAddresses, agents[0].BindAddr)
		}

		config := Config{
			DataDir:            dataDir,
			NodeName:           fmt.Sprintf("Node No.%v", i),
			BindAddr:           addr,
			StartJoinAddresses: startJoinAddresses,
		}

		agent, err := New(config)
		require.NoError(t, err, "create agent error")
		agents = append(agents, agent)
	}
	defer func() {
		for _, agent := range agents {
			err := agent.shutdown()
			require.NoError(t, err, "agent shutdown error")
			require.NoError(t, os.RemoveAll(agent.DataDir))
		}
	}()

	//leaderClient, err := client(agents[0])
	//require.NoError(t, err, "error create leader client")

	//_, err = leaderClient.Produce(
	//	context.Background(),
	//	&api.ProduceRequest{
	//		Record: &api.Record{
	//			Value: []byte("hello"),
	//		},
	//	},
	//)
	//require.NoError(t, err, "leader produce error")
}

func client(agent *Agent) (api.LogClient, error) {
	clientOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	cc, err := grpc.Dial(agent.BindAddr, clientOptions...)
	if err != nil {
		return nil, err
	}
	logClient := api.NewLogClient(cc)

	return logClient, nil
}

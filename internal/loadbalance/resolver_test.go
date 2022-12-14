package loadbalance_test

import (
	"fmt"
	"net"
	"net/url"
	api "proglog/api/v1"
	"proglog/internal/config"
	"proglog/internal/loadbalance"
	"proglog/internal/server"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

func TestResolver(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tlsConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.ServerCertFile,
		KeyFile:       config.ServerKeyFile,
		CAFile:        config.CAFile,
		Server:        true,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	serverCreds := credentials.NewTLS(tlsConfig)

	srv, err := server.NewGrpcServer(&server.Config{
		GetServerer: &getServers{},
	}, grpc.Creds(serverCreds))
	require.NoError(t, err)

	go func() {
		err := srv.Serve(l)
		if err != nil {
			panic(err)
		}
	}()
	// END: setup_test

	// START: mid_test
	conn := &clientConn{}
	require.NoError(t, err)

	clientTls, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.RootClientCertFile,
		KeyFile:       config.RootClientKeyFile,
		CAFile:        config.CAFile,
		Server:        false,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	clientCreds := credentials.NewTLS(clientTls)
	opts := resolver.BuildOptions{
		DialCreds: clientCreds,
	}
	r := &loadbalance.Resolver{}
	targetUrl, err := url.Parse(
		fmt.Sprintf(
			"%s://%s",
			loadbalance.Name,
			l.Addr().String(),
		),
	)

	require.NoError(t, err)

	_, err = r.Build(
		resolver.Target{
			URL: *targetUrl,
		},
		conn,
		opts,
	)
	require.NoError(t, err)
	// END: mid_test

	// START: finish_test
	wantState := resolver.State{
		Addresses: []resolver.Address{
			{
				Addr:       "localhost:9001",
				ServerName: "leader",
				Attributes: attributes.New("is_leader", true),
			},
			{
				Addr:       "localhost:9002",
				ServerName: "follower",
				Attributes: attributes.New("is_leader", false),
			},
		},
	}
	require.Equal(t, wantState.Addresses, conn.state.Addresses)

	conn.state.Addresses = nil
	r.ResolveNow(resolver.ResolveNowOptions{})
	require.Equal(t, wantState, conn.state)
}

// END: finish_test

// START: mock_deps
type getServers struct{}

func (s *getServers) GetServers() ([]*api.Server, error) {
	return []*api.Server{{
		Id:       "leader",
		RpcAddr:  "localhost:9001",
		IsLeader: true,
	}, {
		Id:      "follower",
		RpcAddr: "localhost:9002",
	}}, nil
}

type clientConn struct {
	resolver.ClientConn
	state resolver.State
}

func (c *clientConn) UpdateState(state resolver.State) error {
	c.state = state

	return nil
}

func (c *clientConn) ReportError(error) {}

func (c *clientConn) NewAddress([]resolver.Address) {}

func (c *clientConn) NewServiceConfig(string) {}

func (c *clientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return nil
}

// END: mock_deps

package loadbalance

import (
	"context"
	"fmt"
	api "proglog/api/v1"
	"sync"

	"google.golang.org/grpc/attributes"

	"go.uber.org/zap"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

const Name = "prolog"

// Resolver implements resolver.Builder and resolver.Resolver interfaces.
type Resolver struct {
	mu            sync.Mutex
	clientConn    resolver.ClientConn
	resolverConn  *grpc.ClientConn
	serviceConfig *serviceconfig.ParseResult
	logger        *zap.Logger
}

// START: resolver.Builder

// Build builds the resolver.
func (r *Resolver) Build(target resolver.Target, conn resolver.ClientConn, buildOpts resolver.BuildOptions) (resolver.Resolver, error) {
	r.clientConn = conn
	var opts []grpc.DialOption
	if buildOpts.DialCreds != nil {
		opts = append(opts, grpc.WithTransportCredentials(buildOpts.DialCreds))
	}
	r.serviceConfig = r.clientConn.ParseServiceConfig(
		fmt.Sprintf(`{"loadBalancingConfig":[{"%s":{}}]}`, Name),
	)

	var err error
	r.resolverConn, err = grpc.Dial(target.URL.Path, opts...)
	if err != nil {
		return nil, err
	}

	r.ResolveNow(resolver.ResolveNowOptions{})

	return r, nil
}

func (r *Resolver) Scheme() string {
	return Name
}

// END: resolver.Builder

// START: resolver.Resolver

// ResolveNow will be called by grpc to resolve the target, discover the servers and update the client connection
// with the servers.
func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()
	logClient := api.NewLogClient(r.resolverConn)

	res, err := logClient.GetServers(context.Background(), &api.GetServersRequest{})
	if err != nil {
		r.logger.Error("failed to resolve server", zap.Error(err))
	}

	var addresses []resolver.Address
	for _, server := range res.Servers {
		addresses = append(addresses, resolver.Address{
			Addr:       server.RpcAddr,
			ServerName: server.Id,
			Attributes: attributes.New("is_leader", server.IsLeader),
		})
	}

	err = r.clientConn.UpdateState(resolver.State{
		Addresses:     addresses,
		ServiceConfig: r.serviceConfig,
	})
	if err != nil {
		r.logger.Error("cannot update client connection", zap.Error(err))
	}
}

func (r *Resolver) Close() {
	err := r.resolverConn.Close()
	if err != nil {
		r.logger.Error("failed to close conn", zap.Error(err))
	}
}

// END: resolver.Resolver

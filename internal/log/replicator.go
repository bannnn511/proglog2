package log

import (
	"context"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	api "proglog/api/v1"
	"sync"
)

type replicator struct {
	mu          sync.Mutex
	grpcOption  grpc.DialOption
	localServer api.LogServer
	servers     map[string]chan struct{}
	logger      zap.Logger
	close       chan bool
	leave       chan bool
}

func (r *replicator) Join(name, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// check if server had already joined.
	_, ok := r.servers[name]
	if ok {
		return nil
	}
	r.servers[name] = make(chan struct{})

	go r.replicate(addr, r.servers[name])

	return nil
}

func (r *replicator) Leave(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, ok := r.servers[name]
	if !ok {
		return nil
	}
	close(server)
	delete(r.servers, name)

	return nil
}

func (r *replicator) replicate(addr string, leave chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cc, err := grpc.Dial(addr, r.grpcOption)
	if err != nil {
		r.logError(err, "grpc.Dial() failed", addr)
		return
	}

	logClient := api.NewLogClient(cc)

	stream, err := logClient.ConsumeStream(context.Background(),
		&api.ConsumeRequest{Offset: 0},
	)
	if err != nil {
		r.logError(err, "logClient.ConsumeStream() failed", addr)
		return
	}

	record := make(chan *api.Record)
	go func() {
		for {
			res, err := stream.Recv()
			if err != nil {
				r.logError(err, "failed to receive", addr)
				return
			}

			record <- res.Record
		}
	}()

	for {
		select {
		case <-r.close:
			return
		case <-leave:
			return
		case data := <-record:
			_, err := r.localServer.Produce(context.Background(),
				&api.ProduceRequest{
					Record: data,
				},
			)
			if err != nil {
				r.logError(err, "cannot produce record", addr)
				return
			}
		}
	}
}

func (r *replicator) logError(err error, msg string, addr string) {
	r.logger.Error(
		msg,
		zap.Error(err),
		zap.String("addr", addr),
	)
}

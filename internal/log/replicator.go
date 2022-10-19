package log

import (
	"context"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	api "proglog/api/v1"
	"sync"
)

type Replicator struct {
	mu          sync.Mutex
	DialOptions []grpc.DialOption
	localServer api.LogServer
	servers     map[string]chan struct{}
	logger      *zap.Logger
	close       chan bool
}

// Join appends new server name and address
// then start replicating to local server.
func (r *Replicator) Join(name, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.init()

	// check if server had already joined.
	_, ok := r.servers[name]
	if ok {
		return nil
	}
	r.servers[name] = make(chan struct{})

	go r.replicate(addr, r.servers[name])

	return nil
}

// Leave removes server from list of servers.
func (r *Replicator) Leave(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.init()

	server, ok := r.servers[name]
	if !ok {
		return nil
	}
	close(server)
	delete(r.servers, name)

	return nil
}

// replicate creates a log client and consume stream
// from server address then local server will produce new record from the stream.
func (r *Replicator) replicate(addr string, leave chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cc, err := grpc.Dial(addr, r.DialOptions...)
	if err != nil {
		r.logError(err, "grpc.Dial() failed", addr)
		return
	}

	logClient := api.NewLogClient(cc)

	stream, err := logClient.ConsumeStream(
		context.Background(),
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

// Close stops replicating processes by sending close signal
// to replicate() go routine.
func (r *Replicator) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.close <- true
	close(r.close)
}

func (r *Replicator) logError(err error, msg string, addr string) {
	r.logger.Error(
		msg,
		zap.Error(err),
		zap.String("addr", addr),
	)
}

func (r *Replicator) init() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.servers == nil {
		r.servers = make(map[string]chan struct{})
	}

	if r.logger == nil {
		logger, _ := zap.NewDevelopment()
		r.logger = logger.Named("Replicator")
	}

	if r.close == nil {
		r.close = make(chan bool)
	}
}

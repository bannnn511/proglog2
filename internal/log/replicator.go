package log

import (
	"context"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	api "proglog/api/v1"
	"sync"
	"time"
)

type Replicator struct {
	NodeName    string
	mu          sync.Mutex
	DialOptions []grpc.DialOption
	LocalServer api.LogClient
	servers     map[string]chan struct{}
	logger      *zap.Logger
	close       chan bool
	isClosed    bool
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
	cc, err := grpc.Dial(addr, r.DialOptions...)
	if err != nil {
		r.logError(err, "grpc.Dial() failed", addr)
		return
	}
	defer cc.Close()

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
			//fmt.Println("stream recv", r.NodeName, res.Record.Offset, string(res.Record.Value))

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
			_, err := r.LocalServer.Produce(context.Background(),
				&api.ProduceRequest{
					Record: data,
				},
			)
			if err != nil {
				r.logError(err, "cannot produce record", addr)
				return
			}
			time.Sleep(100 * time.Millisecond)

		}
	}
}

// Close stops replicating processes by sending close signal
// to replicate() go routine.
func (r *Replicator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isClosed {
		return nil
	}
	r.isClosed = true

	close(r.close)

	return nil
}

func (r *Replicator) logError(err error, msg string, addr string) {
	r.logger.Error(
		msg,
		zap.Error(err),
		zap.String("addr", addr),
	)
}

func (r *Replicator) init() {

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

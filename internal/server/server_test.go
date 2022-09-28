package server

import (
	"bytes"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"net"
	"os"
	api "proglog/api/v1"
	"proglog/internal/log"
	"testing"
)

func TestServer(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T,
		client api.LogClient,
		config *Config,
	){
		"produce/consume a message to/from the log succeeeds": testProduceConsume,
		"produce/consume stream succeeds":                     testProduceConsumeStream,
		"consume past log boundary fails":                     testConsumePastBoundary,
	} {
		t.Run(scenario, func(t *testing.T) {
			client, config, teardown := setupTest(t, nil)
			defer teardown()
			fn(t, client, config)
		})
	}
}

func setupTest(t *testing.T, fn func(*Config)) (
	client api.LogClient,
	config *Config,
	teardown func(),
) {
	t.Helper()

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Errorf("net.Listen() error %v", err)
		return
	}

	clientOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	cc, err := grpc.Dial(l.Addr().String(), clientOptions...)
	if err != nil {
		t.Errorf("grpc.Dial() error %v", err)
		return
	}

	dir, err := os.MkdirTemp("", "server-test")
	if err != nil {
		t.Errorf("os.MkdirTemp error %v", err)
		return
	}

	clog, err := log.NewLog(dir, log.Config{})
	if err != nil {
		t.Errorf("log.NewLog() error %v", err)
		return
	}

	config = &Config{
		CommitLog: clog,
	}
	if fn != nil {
		fn(config)
	}
	server, err := NewGrpcServer(config)
	if err != nil {
		t.Errorf("NewGrpcServer error %v", err)
		return
	}
	go func() {
		server.Serve(l)
	}()

	client = api.NewLogClient(cc)

	return client, config, func() {
		server.Stop()
		cc.Close()
		l.Close()
		clog.Remove()
	}
}

func testProduceConsume(t *testing.T, client api.LogClient, config *Config) {
	ctx := context.Background()

	want := &api.Record{
		Value: []byte("hello world"),
	}

	produce, err := client.Produce(
		ctx,
		&api.ProduceRequest{
			Record: want,
		},
	)
	if err != nil {
		t.Errorf("Produce error %v", err)
		return
	}

	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produce.Offset,
	})
	if err != nil {
		t.Errorf("Consume error %v", err)
		return
	}
	if bytes.Compare(want.Value, consume.Record.Value) != 0 {
		t.Errorf("expected %v, got %v", want.Value, consume.Record.Value)
		return
	}
	if want.Offset != consume.Record.Offset {
		t.Errorf("expected %v, got %v", want.Offset, consume.Record.Offset)
		return
	}
}

func testConsumePastBoundary(
	t *testing.T,
	client api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	produce, err := client.Produce(ctx, &api.ProduceRequest{
		Record: &api.Record{
			Value: []byte("hello world"),
		},
	})
	if err != nil {
		t.Errorf("Produce error %v", err)
		return
	}

	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produce.Offset + 1,
	})
	if consume != nil {
		t.Fatal("consume not nil")
	}

	got := status.Code(err)
	want := status.Code(api.ErrorOffsetOfOutRange{}.GRPCStatus().Err())
	if got != want {
		t.Fatalf("got err: %v, want: %v", got, want)
	}
}

func testProduceConsumeStream(
	t *testing.T,
	client api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	records := []*api.Record{{
		Value:  []byte("first message"),
		Offset: 0,
	}, {
		Value:  []byte("second message"),
		Offset: 1,
	}}

	{
		stream, err := client.ProduceStream(ctx)
		if err != nil {
			t.Errorf("ProduceStream() error %v", err)
			return
		}

		for offset, record := range records {
			err = stream.Send(&api.ProduceRequest{
				Record: record,
			})
			if err != nil {
				t.Errorf("stream.Send() error %v", err)
				return
			}

			res, err := stream.Recv()
			if err != nil {
				t.Errorf("stream.Recv() error %v", err)
				return
			}
			if res.Offset != uint64(offset) {
				t.Fatalf(
					"got offset: %d, want: %d",
					res.Offset,
					offset,
				)
			}
		}

	}

	{
		stream, err := client.ConsumeStream(
			ctx,
			&api.ConsumeRequest{Offset: 0},
		)
		if err != nil {
			t.Errorf("ConsumeRequest() error %v", err)
			return
		}

		for i, record := range records {
			res, err := stream.Recv()
			if err != nil {
				t.Errorf("stream.Recv() error %v", err)
				return
			}

			wantRecord := &api.Record{
				Value:  record.Value,
				Offset: uint64(i),
			}
			if bytes.Compare(res.Record.Value, wantRecord.Value) != 0 {
				t.Errorf("want %v, got %v", wantRecord.Value, res.Record.Value)
				return
			}

			if res.Record.Offset != wantRecord.Offset {
				t.Errorf("want %v, got %v", wantRecord.Offset, res.Record.Offset)
				return
			}
		}
	}
}

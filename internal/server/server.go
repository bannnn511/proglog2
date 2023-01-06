package server

import (
	"context"
	"log"
	api "proglog/api/v1"
	"time"

	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpczap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpcctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

type Config struct {
	CommitLog   CommitLog
	GetServerer GetServerer
}

type GetServerer interface {
	GetServers() ([]*api.Server, error)
}

type CommitLog interface {
	Append(record *api.Record) (uint64, error)
	Read(offset uint64) (*api.Record, error)
}

type grpcServer struct {
	api.UnimplementedLogServer
	*Config
}

func newgrpcServer(config *Config) (*grpcServer, error) {
	grpcServer := &grpcServer{
		Config: config,
	}

	return grpcServer, nil
}

func NewGrpcServer(config *Config, grpcOpts ...grpc.ServerOption) (*grpc.Server, error) {
	// START: logger
	zapLogger := zap.L().Named("server")
	zapOpts := []grpczap.Option{
		grpczap.WithDurationField(func(duration time.Duration) zapcore.Field {
			return zap.Int64("grpc.time_ns", duration.Nanoseconds())
		}),
	}
	// END: logger

	// START: trace and metrics
	_ = trace.AlwaysSample()
	if err := view.Register(ocgrpc.DefaultServerViews...); err != nil {
		log.Fatal(err)
	}
	// END: trace and metrics

	// START: grpcOpts
	grpcOpts = append(grpcOpts,
		grpc.StreamInterceptor(
			grpcmiddleware.ChainStreamServer(
				grpcctxtags.StreamServerInterceptor(),
				grpczap.StreamServerInterceptor(zapLogger, zapOpts...),
			),
		),
		grpc.UnaryInterceptor(
			grpcmiddleware.ChainUnaryServer(
				grpcctxtags.UnaryServerInterceptor(),
				grpczap.UnaryServerInterceptor(zapLogger, zapOpts...),
			)),
	)
	grpc.StatsHandler(&ocgrpc.ServerHandler{})
	// ENDs: grpcOpts

	grpcServer := grpc.NewServer(grpcOpts...)
	srv, err := newgrpcServer(config)
	if err != nil {
		return nil, err
	}
	api.RegisterLogServer(grpcServer, srv)

	return grpcServer, nil
}

func (s *grpcServer) Produce(
	_ context.Context,
	req *api.ProduceRequest,
) (*api.ProduceResponse, error) {
	offset, err := s.CommitLog.Append(req.Record)
	if err != nil {
		return nil, err
	}

	return &api.ProduceResponse{Offset: offset}, nil
}

func (s *grpcServer) Consume(
	_ context.Context,
	req *api.ConsumeRequest,
) (*api.ConsumeResponse, error) {
	record, err := s.CommitLog.Read(req.Offset)
	if err != nil {
		return nil, err
	}

	return &api.ConsumeResponse{Record: record}, nil
}

func (s *grpcServer) ProduceStream(
	stream api.Log_ProduceStreamServer,
) error {
	for {
		recv, err := stream.Recv()
		if err != nil {
			return err
		}

		offset, err := s.CommitLog.Append(recv.Record)
		if err != nil {
			return err
		}

		err = stream.Send(&api.ProduceResponse{Offset: offset})
		if err != nil {
			return err
		}
	}
}

func (s *grpcServer) ConsumeStream(
	req *api.ConsumeRequest,
	stream api.Log_ConsumeStreamServer,
) error {
	for {
		select {
		case <-stream.Context().Done():
			return nil
		default:
			res, err := s.Consume(stream.Context(), req)
			switch err.(type) {
			case nil:
			case api.ErrorOffsetOfOutRange:
				continue
			default:
				return err
			}

			if err = stream.Send(res); err != nil {
				return err
			}
			req.Offset++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *grpcServer) GetServers(_ context.Context, req *api.GetServersRequest) (*api.GetServersResponse, error) {
	severs, err := s.GetServerer.GetServers()
	if err != nil {
		return nil, err
	}
	return &api.GetServersResponse{Servers: severs}, nil
}

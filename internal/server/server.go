package server

import (
	"context"
	"google.golang.org/grpc"
	api "proglog/api/v1"
)

type Config struct {
	CommitLog CommitLog
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

func NewGrpcServer(config *Config) (*grpc.Server, error) {
	grpcServer := grpc.NewServer()
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
		}
	}
}

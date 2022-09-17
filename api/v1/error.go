package v1

import (
	"fmt"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
)

type ErrorOffsetOfOutRange struct {
	Offset uint64
}

func (e ErrorOffsetOfOutRange) GRPCStatus() *status.Status {
	st := status.New(
		404,
		fmt.Sprintf("offset out of range: %d", e.Offset),
	)

	msg := fmt.Sprintf(
		"the requested offset is outside the log's range: %d",
		e.Offset,
	)

	d := &errdetails.LocalizedMessage{
		Locale:  "en-US",
		Message: msg,
	}

	std, err := st.WithDetails(d)
	if err != nil {
		return st
	}

	return std
}

func (e ErrorOffsetOfOutRange) Error() string {
	return e.GRPCStatus().Err().Error()
}

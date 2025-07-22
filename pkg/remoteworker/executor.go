package remoteworker

import (
	"context"
	"time"

	remoteworker_pb "bonanza.build/pkg/proto/remoteworker"
)

// Executor is responsible for processing an incoming action, executing
// it, and returning its results. The remote worker client will assume
// that all payloads are byte slices, but decorators can be written to
// transmit things like Protobuf messages.
//
// Implementations of Executor.Execute() should only return an error if
// there are no other mechanisms available to propagate errors to the
// the caller. The reason being that any error returned directly is sent
// back to the client in plain text, as opposed to getting encrypted.
type Executor[TAction, TEvent, TResult any] interface {
	CheckReadiness(ctx context.Context) error
	Execute(ctx context.Context, action TAction, executionTimeout time.Duration, executionEvents chan<- TEvent) (TResult, time.Duration, remoteworker_pb.CurrentState_Completed_Result, error)
}

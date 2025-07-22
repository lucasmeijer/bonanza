package remoteexecution

import (
	"context"
	"crypto/ecdh"
	"iter"

	remoteexecution_pb "bonanza.build/pkg/proto/remoteexecution"
)

// Client for running actions via the remote execution protocol.
//
// For every action that needs to be invoked, an action payload needs to
// be provided. While the action is execution it is possible to access
// execution events, which are provided through an iterator. Upon
// completion, a single result is returned.
type Client[TAction, TEvent, TResult any] interface {
	RunAction(ctx context.Context, platformECDHPublicKey *ecdh.PublicKey, action TAction, actionAdditionalData *remoteexecution_pb.Action_AdditionalData, result *TResult, errOut *error) iter.Seq[TEvent]
}

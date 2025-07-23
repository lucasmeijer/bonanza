package executewithstorage

import (
	"context"
	"time"

	model_core "bonanza.build/pkg/model/core"
	model_core_pb "bonanza.build/pkg/proto/model/core"
	model_executewithstorage_pb "bonanza.build/pkg/proto/model/executewithstorage"
	remoteworker_pb "bonanza.build/pkg/proto/remoteworker"
	"bonanza.build/pkg/remoteworker"
	"bonanza.build/pkg/storage/object"

	"github.com/buildbarn/bb-storage/pkg/util"
)

type executor struct {
	remoteworker.Executor[*Action, model_core.Decodable[object.LocalReference], model_core.Decodable[object.LocalReference]]
}

// NewExecutor creates a decorator for the remote worker executor that
// makes it simpler to run actions on workers that make use of the
// "executewithstorage" protocol (i.e., workers that rely on actions,
// execution events, and results to be backed by object storage).
//
// The advantage of using this decorator is that the caller can use
// native types for decodable references, as opposed to sending and
// receiving their Protobuf message equivalents.
func NewExecutor(
	base remoteworker.Executor[*Action, model_core.Decodable[object.LocalReference], model_core.Decodable[object.LocalReference]],
) remoteworker.Executor[*model_executewithstorage_pb.Action, *model_core_pb.WeakDecodableReference, *model_core_pb.WeakDecodableReference] {
	return &executor{
		Executor: base,
	}
}

func (e *executor) Execute(ctx context.Context, action *model_executewithstorage_pb.Action, executionTimeout time.Duration, executionEventMessages chan<- *model_core_pb.WeakDecodableReference) (*model_core_pb.WeakDecodableReference, time.Duration, remoteworker_pb.CurrentState_Completed_Result, error) {
	namespace, err := object.NewNamespace(action.Namespace)
	if err != nil {
		return nil, 0, 0, util.StatusWrap(err, "Invalid namespace")
	}

	actionReference, err := model_core.NewDecodableLocalReferenceFromWeakProto(
		namespace.ReferenceFormat,
		action.ActionReference,
	)
	if err != nil {
		return nil, 0, 0, util.StatusWrap(err, "Invalid action reference")
	}

	executionEvents := make(chan model_core.Decodable[object.LocalReference], 1)
	storedAllExecutionEvents := make(chan struct{})
	go func() {
		for executionEvent := range executionEvents {
			executionEventMessages <- model_core.DecodableLocalReferenceToWeakProto(executionEvent)
		}
		close(storedAllExecutionEvents)
	}()

	resultReference, virtualExecutionDuration, resultCode, err := e.Executor.Execute(
		ctx,
		&Action{
			Reference: model_core.CopyDecodable(
				actionReference,
				namespace.WithLocalReference(actionReference.Value),
			),
			Encoders: action.ActionEncoders,
			Format:   action.ActionFormat,
		},
		executionTimeout,
		executionEvents,
	)

	close(executionEvents)
	<-storedAllExecutionEvents

	if err != nil {
		return nil, 0, 0, err
	}
	return model_core.DecodableLocalReferenceToWeakProto(resultReference), virtualExecutionDuration, resultCode, nil
}

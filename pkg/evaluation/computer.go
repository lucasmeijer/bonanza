package evaluation

import (
	"context"
	"errors"

	model_core "bonanza.build/pkg/model/core"
	"bonanza.build/pkg/storage/dag"
	"bonanza.build/pkg/storage/object"

	"google.golang.org/protobuf/proto"
)

var ErrMissingDependency = errors.New("missing dependency")

type Computer[TReference, TMetadata any] interface {
	ComputeMessageValue(ctx context.Context, key model_core.Message[proto.Message, TReference], e Environment[TReference, TMetadata]) (model_core.PatchedMessage[proto.Message, dag.ObjectContentsWalker], error)
	ComputeNativeValue(ctx context.Context, key model_core.Message[proto.Message, TReference], e Environment[TReference, TMetadata]) (any, error)
}

type ComputerForTesting Computer[object.LocalReference, model_core.ReferenceMetadata]

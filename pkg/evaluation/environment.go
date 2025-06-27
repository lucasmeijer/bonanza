package evaluation

import (
	model_core "bonanza.build/pkg/model/core"
	"bonanza.build/pkg/storage/dag"

	"google.golang.org/protobuf/proto"
)

type Environment[TReference any, TMetadata any] interface {
	model_core.ObjectManager[TReference, TMetadata]

	GetMessageValue(key model_core.PatchedMessage[proto.Message, dag.ObjectContentsWalker]) model_core.Message[proto.Message, TReference]
	GetNativeValue(key model_core.PatchedMessage[proto.Message, dag.ObjectContentsWalker]) (any, bool)
}

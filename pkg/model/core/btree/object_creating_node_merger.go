package btree

import (
	"github.com/buildbarn/bb-storage/pkg/util"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_encoding "github.com/buildbarn/bonanza/pkg/model/encoding"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/protobuf/proto"
)

// ParentNodeComputer can be used by ObjectCreatingNodeMerger to combine
// the values of nodes stored in an object into a single node that can
// be stored in its parent.
type ParentNodeComputer[TNode proto.Message, TMetadata model_core.ReferenceMetadata] func(
	createdObject model_core.Decodable[model_core.CreatedObject[TMetadata]],
	childNodes []TNode,
) (model_core.PatchedMessage[TNode, TMetadata], error)

// NewObjectCreatingNodeMerger creates a NodeMerger that can be used in
// combination with Builder to construct B-trees that are backed by
// storage objects that reference each other.
func NewObjectCreatingNodeMerger[TNode proto.Message, TMetadata model_core.ReferenceMetadata](encoder model_encoding.BinaryEncoder, referenceFormat object.ReferenceFormat, parentNodeComputer ParentNodeComputer[TNode, TMetadata]) NodeMerger[TNode, TMetadata] {
	return func(list model_core.PatchedMessage[[]TNode, TMetadata]) (model_core.PatchedMessage[TNode, TMetadata], error) {
		// Marshal each of the messages, prepending its size.
		createdObject, err := model_core.MarshalAndEncodePatchedListMessage(list, referenceFormat, encoder)
		if err != nil {
			return model_core.PatchedMessage[TNode, TMetadata]{}, util.StatusWrap(err, "Failed to marshal list")
		}

		// Construct a parent node that references the object containing
		// the children.
		parentNode, err := parentNodeComputer(createdObject, list.Message)
		if err != nil {
			return model_core.PatchedMessage[TNode, TMetadata]{}, util.StatusWrap(err, "Failed to compute parent node")
		}
		return parentNode, nil
	}
}

// ParentNodeComputerForTesting is an instantiation of
// ParentNodeComputer for generating mocks to be used by tests.
type ParentNodeComputerForTesting ParentNodeComputer[*model_filesystem_pb.FileContents, model_core.ReferenceMetadata]

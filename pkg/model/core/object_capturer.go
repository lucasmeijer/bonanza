package core

import (
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/protobuf/proto"
)

// CreatedObjectCapturer can be used as a factory type for reference
// metadata. Given the contents of an object and the metadata of all of
// its children, it may yield new metadata.
type CreatedObjectCapturer[TMetadata any] interface {
	CaptureCreatedObject(CreatedObject[TMetadata]) TMetadata
}

type walkableCreatedObjectCapturer struct{}

// WalkableCreatedObjectCapturer is an implementation of ObjectCapturer
// that creates a dag.ObjectContentsWalker for each created object. This
// ends up keeping objects in memory and only allows them to be
// traversed as part of the upload process.
//
// This implementation is sufficient when given existing Merkle trees in
// the form of a dag.ObjectContentsWalkers that need to be combined into
// single Merkle tree.
var WalkableCreatedObjectCapturer CreatedObjectCapturer[dag.ObjectContentsWalker] = walkableCreatedObjectCapturer{}

func (walkableCreatedObjectCapturer) CaptureCreatedObject(createdObject CreatedObject[dag.ObjectContentsWalker]) dag.ObjectContentsWalker {
	return dag.NewSimpleObjectContentsWalker(createdObject.Contents, createdObject.Metadata)
}

// ExistingObjectCapturer can be used as a factory type for reference
// metadata. Given a reference of an object that already exists ins
// torage, it may yield metadata.
type ExistingObjectCapturer[TReference, TMetadata any] interface {
	CaptureExistingObject(TReference) TMetadata
}

// ObjectCapturer is a combination of CreatedObjectCapturer and
// ExistingObjectCapturer, allowing the construction of metadata both
// for newly created objects and ones that exist in storage.
type ObjectCapturer[TReference, TMetadata any] interface {
	CreatedObjectCapturer[TMetadata]
	ExistingObjectCapturer[TReference, TMetadata]
}

// NewPatchedMessageFromExistingCaptured is identical to
// NewPatchedMessageFromExisting, except that it automatically creates
// metadata for all references contained the message using the provided
// capturer.
func NewPatchedMessageFromExistingCaptured[
	TMessage any,
	TMetadata ReferenceMetadata,
	TMessagePtr interface {
		*TMessage
		proto.Message
	},
	TReference object.BasicReference,
](
	capturer ExistingObjectCapturer[TReference, TMetadata],
	m Message[TMessagePtr, TReference],
) PatchedMessage[TMessagePtr, TMetadata] {
	return NewPatchedMessageFromExisting(
		m,
		func(index int) TMetadata {
			return capturer.CaptureExistingObject(
				m.OutgoingReferences.GetOutgoingReference(index),
			)
		},
	)
}

type ObjectReferencer[TReference, TMetadata any] interface {
	ReferenceObject(object.LocalReference, TMetadata) TReference
}

func NewMessageFromPatchedReferenced[TMessage, TReference any, TMetadata ReferenceMetadata](
	referencer ObjectReferencer[TReference, TMetadata],
	m PatchedMessage[TMessage, TMetadata],
) Message[TMessage, TReference] {
	references, metadata := m.Patcher.SortAndSetReferences()
	outgoingReferences := make(object.OutgoingReferencesList[TReference], 0, len(metadata))
	for i, m := range metadata {
		outgoingReferences = append(
			outgoingReferences,
			referencer.ReferenceObject(references.GetOutgoingReference(i), m),
		)
	}
	return NewMessage(m.Message, outgoingReferences)
}

// ObjectManager is an extension to ObjectCapturer, allowing metadata to
// be converted back to references. This can be of use in environments
// where objects also need to be accessible for reading right after they
// have been constructed, without explicitly waiting for them to be
// written to storage.
type ObjectManager[TReference, TMetadata any] interface {
	ObjectCapturer[TReference, TMetadata]
	ObjectReferencer[TReference, TMetadata]
}

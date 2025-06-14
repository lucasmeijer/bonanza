package core

import (
	"math"

	model_encoding "github.com/buildbarn/bonanza/pkg/model/encoding"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/protobuf/proto"
)

// PatchedMessage is a tuple for storing a Protobuf message that
// contains model_core_pb.Reference messages, and the associated
// ReferenceMessagePatcher that can be used to assign indices to these
// references.
type PatchedMessage[TMessage any, TMetadata ReferenceMetadata] struct {
	Message TMessage
	Patcher *ReferenceMessagePatcher[TMetadata]
}

// NewPatchedMessage creates a PatchedMessage, given an existing Protobuf
// message and reference message patcher.
func NewPatchedMessage[TMessage any, TMetadata ReferenceMetadata](
	message TMessage,
	patcher *ReferenceMessagePatcher[TMetadata],
) PatchedMessage[TMessage, TMetadata] {
	return PatchedMessage[TMessage, TMetadata]{
		Message: message,
		Patcher: patcher,
	}
}

// NewPatchedMessageFromExisting creates a PatchedMessage, given an
// existing Protobuf message that may contain one or more references to
// other objects. For each reference that is found, a callback is
// invoked to create metadata to associate with the reference.
func NewPatchedMessageFromExisting[
	TMessage proto.Message,
	TMetadata ReferenceMetadata,
	TReference object.BasicReference,
](
	existing Message[TMessage, TReference],
	createMetadata ReferenceMetadataCreator[TMetadata],
) PatchedMessage[TMessage, TMetadata] {
	return BuildPatchedMessage(func(patcher *ReferenceMessagePatcher[TMetadata]) TMessage {
		if existing.OutgoingReferences.GetDegree() == 0 {
			return existing.Message
		}

		clonedMessage := proto.Clone(existing.Message)
		a := referenceMessageAdder[TMetadata, TReference]{
			patcher:            patcher,
			outgoingReferences: existing.OutgoingReferences,
			createMetadata:     createMetadata,
		}
		a.addReferenceMessagesRecursively(clonedMessage.ProtoReflect())
		return clonedMessage.(TMessage)
	})
}

// NewSimplePatchedMessage is a helper function for creating instances
// of PatchedMessage for messages that don't contain any references.
func NewSimplePatchedMessage[TMetadata ReferenceMetadata, TMessage any](v TMessage) PatchedMessage[TMessage, TMetadata] {
	return NewPatchedMessage(v, NewReferenceMessagePatcher[TMetadata]())
}

// BuildPatchedMessage is a convenience function for constructing
// PatchedMessage that has a newly created ReferenceMessagePatcher
// attached to it. A callback is invoked for creating a message that is
// managed by the ReferenceMessagePatcher.
func BuildPatchedMessage[TMessage any, TMetadata ReferenceMetadata](
	builder func(*ReferenceMessagePatcher[TMetadata]) TMessage,
) PatchedMessage[TMessage, TMetadata] {
	patcher := NewReferenceMessagePatcher[TMetadata]()
	return NewPatchedMessage(builder(patcher), patcher)
}

// IsSet returns true if the PatchedMessage is assigned to a message
// and its associated reference message patcher.
func (m PatchedMessage[T, TMetadata]) IsSet() bool {
	return m.Patcher != nil
}

// Clear the instance of PatchedMessage, disassociating it from its
// message and reference message patcher. The reference message patcher
// is not discarded, meaning that any resources owned by reference
// metadata is not released.
func (m *PatchedMessage[T, TMetadata]) Clear() {
	*m = PatchedMessage[T, TMetadata]{}
}

// Discard the reference message patcher, releasing any resources owned
// by reference metadata.
func (m *PatchedMessage[T, TMetadata]) Discard() {
	if m.Patcher != nil {
		m.Patcher.Discard()
	}
	m.Clear()
}

// SortAndSetReferences assigns indices to outgoing references.
func (m PatchedMessage[T, TMetadata]) SortAndSetReferences() (TopLevelMessage[T, object.LocalReference], []TMetadata) {
	references, metadata := m.Patcher.SortAndSetReferences()
	return NewTopLevelMessage(m.Message, references), metadata
}

// MarshalAndEncode marshals a patched message, encodes it, and converts
// it to an object that can be written to storage.
func MarshalAndEncode[TMetadata ReferenceMetadata](
	m PatchedMessage[Marshalable, TMetadata],
	referenceFormat object.ReferenceFormat,
	encoder model_encoding.BinaryEncoder,
) (Decodable[CreatedObject[TMetadata]], error) {
	references, metadata := m.Patcher.SortAndSetReferences()
	data, err := m.Message.Marshal()
	if err != nil {
		return Decodable[CreatedObject[TMetadata]]{}, err
	}
	encodedData, decodingParameters, err := encoder.EncodeBinary(data)
	if err != nil {
		return Decodable[CreatedObject[TMetadata]]{}, err
	}
	contents, err := referenceFormat.NewContents(references, encodedData)
	if err != nil {
		return Decodable[CreatedObject[TMetadata]]{}, err
	}
	return NewDecodable(
		CreatedObject[TMetadata]{
			Contents: contents,
			Metadata: metadata,
		},
		decodingParameters,
	), nil
}

// MarshalAny wraps a patched message into a model_core_pb.Any message.
// References stored in the original message are kept intact.
func MarshalAny[TMessage proto.Message, TMetadata ReferenceMetadata](m PatchedMessage[TMessage, TMetadata]) (PatchedMessage[*model_core_pb.Any, TMetadata], error) {
	topLevelMessage, metadata := m.SortAndSetReferences()
	anyMessage, err := MarshalTopLevelAny(topLevelMessage)
	if err != nil {
		return PatchedMessage[*model_core_pb.Any, TMetadata]{}, err
	}

	// Construct a table for remapping references contained in the
	// message to indices on the other message.
	degree := anyMessage.OutgoingReferences.GetDegree()
	indices := make([]uint32, 0, degree)
	for i := 0; i < degree; i++ {
		indices = append(indices, math.MaxUint32)
	}

	// Create a new reference message patcher that contains the same
	// references and metadata as the original message, but instead
	// managing the table used for remapping.
	newPatcher := &ReferenceMessagePatcher[TMetadata]{
		messagesByReference: make(map[object.LocalReference]referenceMessages[TMetadata], degree),
		height:              m.Patcher.height,
	}
	for i := 0; i < degree; i++ {
		newPatcher.messagesByReference[anyMessage.OutgoingReferences.GetOutgoingReference(i)] = referenceMessages[TMetadata]{
			metadata: metadata[i],
			indices:  []*uint32{&indices[i]},
		}
	}

	return NewPatchedMessage(
		&model_core_pb.Any{
			Value: anyMessage.Message,
			References: &model_core_pb.ReferenceSet{
				Indices: indices,
			},
		},
		newPatcher,
	), nil
}

func MessageToMarshalable[TMessage proto.Message, TMetadata ReferenceMetadata](m PatchedMessage[TMessage, TMetadata]) PatchedMessage[Marshalable, TMetadata] {
	return NewPatchedMessage(
		NewMessageMarshalable(m.Message),
		m.Patcher,
	)
}

func MessageListToMarshalable[TMessage proto.Message, TMetadata ReferenceMetadata](m PatchedMessage[[]TMessage, TMetadata]) PatchedMessage[Marshalable, TMetadata] {
	return NewPatchedMessage(
		NewMessageListMarshalable(m.Message),
		m.Patcher,
	)
}

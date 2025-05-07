package core

import (
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/protobuf/proto"
)

// TopLevelMessage is identical to Message, except that it may only
// refer to a message for which all embedded references are
// consecutively numbered, and the outgoing references list contains no
// references in addition to the ones embedded in the message.
//
// As top-level messages are considered being canonical, this type
// provides some additional methods for generating keys and performing
// comparisons.
type TopLevelMessage[TMessage any, TReference any] struct {
	Message            TMessage
	OutgoingReferences object.OutgoingReferences[TReference]
}

// MessagesEqual returns true if two top-level messages contain the same
// data. This function can only be called against top-level messages, as
// only those consistently number any outgoing references.
func MessagesEqual[
	TMessage proto.Message,
	TReference1, TReference2 object.BasicReference,
](m1 TopLevelMessage[TMessage, TReference1], m2 TopLevelMessage[TMessage, TReference2]) bool {
	degree1, degree2 := m1.OutgoingReferences.GetDegree(), m2.OutgoingReferences.GetDegree()
	if degree1 != degree2 {
		return false
	}
	for i := 0; i < degree1; i++ {
		if m1.OutgoingReferences.GetOutgoingReference(i).GetLocalReference() != m2.OutgoingReferences.GetOutgoingReference(i).GetLocalReference() {
			return false
		}
	}
	return proto.Equal(m1.Message, m2.Message)
}

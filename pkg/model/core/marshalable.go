package core

import (
	"github.com/buildbarn/bonanza/pkg/encoding/varint"

	"google.golang.org/protobuf/proto"
)

// Marshalable represents a type of value that can be stored in an
// object.
//
// This type is typically used in contexts where a
// PatchedMessage[TMessage, TMetadata] needs to be passed along, but no
// assumptions may be made about the message type. In some of those
// cases it's possible to use PatchedMessage[proto.Message, TMetadata],
// but that wouldn't work if TMessage is a list. This can be solved by
// using PatchedMessage[Marshalable, TMetadata] instead.
type Marshalable interface {
	Marshal() ([]byte, error)
}

type messageMarshalable struct {
	message proto.Message
}

// NewMessageMarshalable converts a Protobuf message to a Marshalable.
// This prevents further access to the message's contents, but does
// allow it to be passed along in contexts where the data is only
// expected to be marshaled and stored.
func NewMessageMarshalable(message proto.Message) Marshalable {
	return messageMarshalable{
		message: message,
	}
}

var marshalOptions = proto.MarshalOptions{
	Deterministic: true,
	UseCachedSize: true,
}

func (mm messageMarshalable) Marshal() ([]byte, error) {
	return marshalOptions.Marshal(mm.message)
}

type messageListMarshalable[TMessage proto.Message] struct {
	messages []TMessage
}

// NewMessageListMarshalable converts a list of Protobuf message to a
// Marshalable. This prevents further access to the message's contents,
// but does allow it to be passed along in contexts where the data is
// only expected to be marshaled and stored.
//
// When marshaled, each message is prefixed with its size.
func NewMessageListMarshalable[TMessage proto.Message](messages []TMessage) Marshalable {
	return messageListMarshalable[TMessage]{
		messages: messages,
	}
}

func (mlm messageListMarshalable[TMessage]) Marshal() ([]byte, error) {
	var data []byte
	for _, message := range mlm.messages {
		data = varint.AppendForward(data, marshalOptions.Size(message))
		var err error
		data, err = marshalOptions.MarshalAppend(data, message)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

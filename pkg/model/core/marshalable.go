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

type protoMarshalable struct {
	message proto.Message
}

// NewProtoMarshalable converts a Protobuf message to a Marshalable.
// This prevents further access to the message's contents, but does
// allow it to be passed along in contexts where the data is only
// expected to be marshaled and stored.
func NewProtoMarshalable(message proto.Message) Marshalable {
	return protoMarshalable{
		message: message,
	}
}

var marshalOptions = proto.MarshalOptions{
	Deterministic: true,
	UseCachedSize: true,
}

func (mm protoMarshalable) Marshal() ([]byte, error) {
	return marshalOptions.Marshal(mm.message)
}

type protoListMarshalable[TMessage proto.Message] struct {
	messages []TMessage
}

// NewProtoListMarshalable converts a list of Protobuf message to a
// Marshalable. This prevents further access to the message's contents,
// but does allow it to be passed along in contexts where the data is
// only expected to be marshaled and stored.
//
// When marshaled, each message is prefixed with its size.
func NewProtoListMarshalable[TMessage proto.Message](messages []TMessage) Marshalable {
	return protoListMarshalable[TMessage]{
		messages: messages,
	}
}

func (mlm protoListMarshalable[TMessage]) Marshal() ([]byte, error) {
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

type rawMarshalable struct {
	data []byte
}

func NewRawMarshalable(data []byte) Marshalable {
	return rawMarshalable{
		data: data,
	}
}

func (rm rawMarshalable) Marshal() ([]byte, error) {
	return rm.data, nil
}

package core

import (
	"encoding/base64"
	"strings"

	model_core_pb "bonanza.build/pkg/proto/model/core"
	"bonanza.build/pkg/storage/object"

	"github.com/buildbarn/bb-storage/pkg/util"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Decodable can be used to annotate an object with parameters that are
// used to decode an object. For example, Decodable[CreatedObject[T]]
// can hold a created object that can subsequently be accessed.
// Similarly, Decodable[object.LocalReference] can be used to refer to
// an object that can subsequently be accessed.
//
// Decodable is comparable if its value is comparable as well.
type Decodable[T any] struct {
	Value T

	//Lets use a fixed-size array for decoding parameters, so Decodable[] can be marshalled, and used as a key in a map.
	//We know actual decoding parameters are always <=16 length.
	decodingParameters       [16]byte
	decodingParametersLength uint8
}

// NewDecodable is a helper function for creating instances of
// Decodable[T].
func NewDecodable[T any](value T, decodingParameters []byte) Decodable[T] {
	var d Decodable[T]
	d.Value = value
	if len(decodingParameters) > 16 {
		panic("decoding parameters exceed maximum size of 16 bytes")
	}
	d.decodingParametersLength = uint8(len(decodingParameters))
	copy(d.decodingParameters[:], decodingParameters)
	return d
}

// GetDecodingParameters returns the parameters needed to decode the
// object associated with the value.
func (d *Decodable[T]) GetDecodingParameters() []byte {
	return d.decodingParameters[:d.decodingParametersLength]
}

// CopyDecodable extracts the decoding parameters of a given
// Decodable[T] and attaches it to another object.
func CopyDecodable[T1, T2 any](from Decodable[T1], to T2) Decodable[T2] {
	return Decodable[T2]{
		Value:                    to,
		decodingParameters:       from.decodingParameters,
		decodingParametersLength: from.decodingParametersLength,
	}
}

// NewDecodableLocalReferenceFromString converts a string of the format
// "${reference}.${decodingParameters}", where both parts are base64
// encoded, to a decodable local reference. Strings of this format may
// be printed as part of logs and other diagnostic output.
func NewDecodableLocalReferenceFromString(referenceFormat object.ReferenceFormat, s string) (Decodable[object.LocalReference], error) {
	var bad Decodable[object.LocalReference]
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return bad, status.Error(codes.InvalidArgument, "Missing \".\" separator")
	}

	rawReference, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return bad, util.StatusWrapWithCode(err, codes.InvalidArgument, "Invalid reference")
	}
	localReference, err := referenceFormat.NewLocalReference(rawReference)
	if err != nil {
		return bad, util.StatusWrapWithCode(err, codes.InvalidArgument, "Invalid reference")
	}

	decodingParameters, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return bad, util.StatusWrapWithCode(err, codes.InvalidArgument, "Invalid decoding parameters")
	}

	return NewDecodable(localReference, decodingParameters), nil
}

// DecodableLocalReferenceToString converts a reference containing
// decoding parameters to a string of the format
// "${reference}.${decodingParameters}", where both parts are base64.
// Strings of this format may be printed as part of logs and other
// diagnostic output.
func DecodableLocalReferenceToString[TReference object.BasicReference](reference Decodable[TReference]) string {
	rawReference := reference.Value.GetRawReference()
	decodingParameters := reference.GetDecodingParameters()
	buf := make([]byte, 0, base64.RawURLEncoding.EncodedLen(len(rawReference))+1+base64.RawURLEncoding.EncodedLen(len(decodingParameters)))
	buf = base64.RawURLEncoding.AppendEncode(buf, rawReference)
	buf = append(buf, '.')
	buf = base64.RawURLEncoding.AppendEncode(buf, decodingParameters)
	return string(buf)
}

// NewDecodableLocalReferenceFromWeakProto converts a
// WeakDecodableReference Protobuf messages to its native counterpart.
func NewDecodableLocalReferenceFromWeakProto(referenceFormat object.ReferenceFormat, m *model_core_pb.WeakDecodableReference) (Decodable[object.LocalReference], error) {
	localReference, err := referenceFormat.NewLocalReference(m.GetReference())
	if err != nil {
		return Decodable[object.LocalReference]{}, err
	}
	return NewDecodable(localReference, m.DecodingParameters), nil
}

// DecodableLocalReferenceToWeakProto converts a reference containing
// decoding parameters to a Protobuf message, so that it may be used as
// a weak reference as part of RPC request/response bodies or objects in
// storage.
func DecodableLocalReferenceToWeakProto[TReference object.BasicReference](reference Decodable[TReference]) *model_core_pb.WeakDecodableReference {
	return &model_core_pb.WeakDecodableReference{
		Reference:          reference.Value.GetRawReference(),
		DecodingParameters: reference.GetDecodingParameters(),
	}
}

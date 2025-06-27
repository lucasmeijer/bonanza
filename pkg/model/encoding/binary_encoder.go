package encoding

import (
	"crypto/aes"

	"bonanza.build/pkg/proto/model/encoding"

	"github.com/buildbarn/bb-storage/pkg/util"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BinaryEncoder can be used to encode binary data. Examples of encoding
// steps include compression and encryption. These encoding steps must
// be reversible.
//
// Many applications give a special meaning to empty data (e.g., the
// default value of bytes fields in a Protobuf message being). Because
// of that, implementations of BinaryEncoder should ensure that empty
// data should remain empty when encoded.
type BinaryEncoder interface {
	EncodeBinary(in []byte) ([]byte, []byte, error)
	DecodeBinary(in, parameters []byte) ([]byte, error)
	GetDecodingParametersSizeBytes() int
}

// NewBinaryEncoderFromProto creates a BinaryEncoder that behaves
// according to the specification provided in the form of a Protobuf
// message.
func NewBinaryEncoderFromProto(configurations []*encoding.BinaryEncoder, maximumDecodedSizeBytes uint32) (BinaryEncoder, error) {
	encoders := make([]BinaryEncoder, 0, len(configurations))
	for _, configuration := range configurations {
		switch encoderConfiguration := configuration.Encoder.(type) {
		case *encoding.BinaryEncoder_LzwCompressing:
			encoders = append(
				encoders,
				NewLZWCompressingBinaryEncoder(maximumDecodedSizeBytes),
			)
		case *encoding.BinaryEncoder_DeterministicEncrypting:
			encryptionKey, err := aes.NewCipher(encoderConfiguration.DeterministicEncrypting.EncryptionKey)
			if err != nil {
				return nil, util.StatusWrapWithCode(err, codes.InvalidArgument, "Invalid encryption key")
			}
			encoders = append(
				encoders,
				NewDeterministicEncryptingBinaryEncoder(encryptionKey),
			)
		default:
			return nil, status.Error(codes.InvalidArgument, "Unknown binary encoder type")
		}
	}
	return NewChainedBinaryEncoder(encoders), nil
}

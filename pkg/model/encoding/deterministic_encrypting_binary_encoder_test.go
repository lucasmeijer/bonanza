package encoding_test

import (
	"crypto/aes"
	"crypto/rand"
	"testing"

	"bonanza.build/pkg/model/encoding"

	"github.com/buildbarn/bb-storage/pkg/testutil"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDeterministicEncryptingBinaryEncoder(t *testing.T) {
	blockCipher, err := aes.NewCipher([]byte{
		0x10, 0xc2, 0xfe, 0xfd, 0x4b, 0x11, 0x43, 0x9b,
		0x1e, 0x73, 0xda, 0x12, 0xef, 0x53, 0xd1, 0x31,
		0xf7, 0x7e, 0x6c, 0xb8, 0x2f, 0xdf, 0x79, 0xb0,
		0x90, 0x6b, 0x23, 0x3c, 0x4a, 0x32, 0xa0, 0x14,
	})
	require.NoError(t, err)
	binaryEncoder := encoding.NewDeterministicEncryptingBinaryEncoder(blockCipher)

	t.Run("EncodeBinary", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			encodedData, initializationVector, err := binaryEncoder.EncodeBinary(nil)
			require.NoError(t, err)
			require.Empty(t, encodedData)
			require.Equal(t, []byte{
				0x23, 0x35, 0x89, 0xdf, 0x0e, 0xf7, 0xe4, 0xd6,
				0x0b, 0xb5, 0x21, 0xfd, 0x18, 0x50, 0xc1, 0xce,
			}, initializationVector)
		})

		t.Run("HelloWorld", func(t *testing.T) {
			encodedData, initializationVector, err := binaryEncoder.EncodeBinary([]byte("Hello world"))
			require.NoError(t, err)
			require.Equal(t, []byte{
				// Encrypted payload.
				0x31, 0x9c, 0x21, 0xeb, 0x5a, 0x33, 0xd0, 0xb7,
				0x8b, 0xde, 0x31,
				// Padding.
				0x03,
			}, encodedData)
			require.Equal(t, []byte{
				0x10, 0x23, 0x7d, 0x7d, 0xe1, 0x80, 0x2e, 0xe0,
				0x7c, 0x0c, 0xbe, 0x21, 0x53, 0x9d, 0x7e, 0xde,
			}, initializationVector)
		})
	})

	t.Run("DecodeBinary", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			decodedData, err := binaryEncoder.DecodeBinary(
				/* in = */ nil,
				/* initializationVector = */ []byte{
					0x7e, 0x2d, 0x56, 0x18, 0xc7, 0x3b, 0xda, 0x73,
					0x0a, 0x97, 0xef, 0xfd, 0xaf, 0x6d, 0xec, 0x96,
				},
			)
			require.NoError(t, err)
			require.Empty(t, decodedData)
		})

		t.Run("HelloWorld", func(t *testing.T) {
			decodedData, err := binaryEncoder.DecodeBinary(
				[]byte{
					// Encrypted payload.
					0x31, 0x9c, 0x21, 0xeb, 0x5a, 0x33, 0xd0, 0xb7,
					0x8b, 0xde, 0x31,
					// Padding.
					0x03,
				},
				/* initializationVector = */ []byte{
					0x10, 0x23, 0x7d, 0x7d, 0xe1, 0x80, 0x2e, 0xe0,
					0x7c, 0x0c, 0xbe, 0x21, 0x53, 0x9d, 0x7e, 0xde,
				},
			)
			require.NoError(t, err)
			require.Equal(t, []byte("Hello world"), decodedData)
		})

		t.Run("DecodingParametersTooShort", func(t *testing.T) {
			_, err := binaryEncoder.DecodeBinary([]byte("Hello"), nil)
			testutil.RequireEqualStatus(t, status.Error(codes.InvalidArgument, "Decoding parameters are 0 bytes in size, while the initialization vector was expected to be 16 bytes in size"), err)
		})

		t.Run("BadPadding", func(t *testing.T) {
			_, err := binaryEncoder.DecodeBinary(
				[]byte{
					// Padding.
					0x60, 0x94, 0x5b, 0x87, 0x83, 0x5d, 0xeb, 0xd2,
					0x11, 0x4a, 0x9e, 0xa3, 0x0c, 0xf3, 0x4d, 0xba,
				},
				/* initializationVector = */ []byte{
					0x3a, 0xd1, 0xc5, 0xdc, 0xd0, 0x85, 0xf3, 0xd1,
					0xac, 0x1e, 0xaf, 0xd1, 0xe3, 0x92, 0x0d, 0x92,
				},
			)
			testutil.RequireEqualStatus(t, status.Error(codes.InvalidArgument, "Padding contains invalid byte with value 118"), err)
		})

		t.Run("TooMuchPadding", func(t *testing.T) {
			// Additional check to ensure that other
			// implementations encode data the same way.
			// Using different amounts of padding may
			// introduce information leakage.
			_, err := binaryEncoder.DecodeBinary(
				[]byte{
					// Encrypted payload.
					0x31, 0x9c, 0x21, 0xeb, 0x5a, 0x33, 0xd0, 0xb7,
					0x8b, 0xde, 0x31,
					// Padding.
					0x03, 0x44,
				},
				/* initializationVector = */ []byte{
					0x10, 0x23, 0x7d, 0x7d, 0xe1, 0x80, 0x2e, 0xe0,
					0x7c, 0x0c, 0xbe, 0x21, 0x53, 0x9d, 0x7e, 0xde,
				},
			)
			testutil.RequireEqualStatus(t, status.Error(codes.InvalidArgument, "Encoded data is 13 bytes in size, while 12 bytes were expected for a payload of 11 bytes"), err)
		})
	})

	t.Run("RandomEncodeDecode", func(t *testing.T) {
		original := make([]byte, 10000)
		for length := 0; length < len(original); length++ {
			n, err := rand.Read(original[:length])
			require.NoError(t, err)
			require.Equal(t, length, n)

			encoded, initializationVector, err := binaryEncoder.EncodeBinary(original[:length])
			require.NoError(t, err)

			decoded, err := binaryEncoder.DecodeBinary(encoded, initializationVector)
			require.NoError(t, err)
			require.Equal(t, original[:length], decoded)
		}
	})
}

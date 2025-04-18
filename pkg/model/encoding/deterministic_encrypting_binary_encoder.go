package encoding

import (
	"crypto/cipher"
	"crypto/sha256"
	"math/bits"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type deterministicEncryptingBinaryEncoder struct {
	blockCipher    cipher.Block
	blockSizeBytes int
}

// NewDeterministicEncryptingBinaryEncoder creates a BinaryEncoder that
// is capable of encrypting and decrypting data. The encryption process
// is deterministic, in that encrypting the same data twice results in
// the same encoded version of the data. It does not use Authenticating
// Encryption (AE), meaning that other facilities need to be present to
// ensure integrity of data.
func NewDeterministicEncryptingBinaryEncoder(blockCipher cipher.Block) BinaryEncoder {
	return &deterministicEncryptingBinaryEncoder{
		blockCipher:    blockCipher,
		blockSizeBytes: blockCipher.BlockSize(),
	}
}

// getPaddedSizeBytes computes the size of the encrypted output, with
// padding in place. Because we use Counter (CTR) mode, we don't need
// any padding to encrypt the data itself. However, adding it reduces
// information leakage by obfuscating the original size.
//
// Use the same structure as Padded Uniform Random Blobs (PURBs), where
// the length is rounded up to a floating point number whose mantissa is
// no longer than its exponent.
//
// More details: Reducing Metadata Leakage from Encrypted Files and
// Communication with PURBs, Algorithm 1 "PADMÉ".
// https://petsymposium.org/popets/2019/popets-2019-0056.pdf
func getPaddedSizeBytes(dataSizeBytes int) int {
	e := bits.Len(uint(dataSizeBytes)) - 1
	bitsToClear := e - bits.Len(uint(e))
	return (dataSizeBytes>>bitsToClear + 1) << bitsToClear
}

func (be *deterministicEncryptingBinaryEncoder) EncodeBinary(in []byte) ([]byte, []byte, error) {
	// Pick an initialization vector. Because this has to work
	// deterministically, hash the input. Encrypt it, so that the
	// hash itself isn't revealed. That would allow fingerprinting
	// of objects, even if the key is changed.
	ivHash := sha256.Sum256(in)
	initializationVector := make([]byte, be.blockSizeBytes)
	be.blockCipher.Encrypt(initializationVector, ivHash[:be.blockSizeBytes])

	if len(in) == 0 {
		return []byte{}, initializationVector, nil
	}

	out := make([]byte, getPaddedSizeBytes(len(in)))
	stream := cipher.NewCTR(be.blockCipher, initializationVector)
	stream.XORKeyStream(out, in)

	outPadding := out[len(in):]
	outPadding[0] = 0x80
	stream.XORKeyStream(outPadding, outPadding)
	return out, initializationVector, nil
}

func (be *deterministicEncryptingBinaryEncoder) DecodeBinary(in, initializationVector []byte) ([]byte, error) {
	if len(initializationVector) != be.blockSizeBytes {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Decoding parameters are %d bytes in size, while the initialization vector was expected to be %d bytes in size",
			len(initializationVector),
			be.blockSizeBytes,
		)
	}

	if len(in) == 0 {
		return []byte{}, nil
	}

	// Decrypt the data, using the initialization vector that is
	// stored before it.
	out := make([]byte, len(in))
	stream := cipher.NewCTR(be.blockCipher, initializationVector)
	stream.XORKeyStream(out, in)

	// Remove trailing padding.
	for l := len(out) - 1; l > 0; l-- {
		switch out[l] {
		case 0x00:
		case 0x80:
			out = out[:l]
			if encodedSizeBytes := getPaddedSizeBytes(len(out)); len(in) != encodedSizeBytes {
				return nil, status.Errorf(
					codes.InvalidArgument,
					"Encoded data is %d bytes in size, while %d bytes were expected for a payload of %d bytes",
					len(in),
					encodedSizeBytes,
					len(out),
				)
			}
			return out, nil
		default:
			return nil, status.Errorf(codes.InvalidArgument, "Padding contains invalid byte with value %d", int(out[l]))
		}
	}
	return nil, status.Error(codes.InvalidArgument, "No data remains after removing padding")
}

func (be *deterministicEncryptingBinaryEncoder) GetDecodingParametersSizeBytes() int {
	return be.blockSizeBytes
}

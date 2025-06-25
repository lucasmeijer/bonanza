package object

import (
	"encoding/binary"
	"encoding/hex"
)

// FlatReference is a simplified version of LocalReference that assumes
// that the height and degree of an object are zero. This means that
// only the hash and size need to be tracked, similar to REv2's Digest
// message.
//
// Because the hash and size are stored at the beginning of a
// LocalReference, FlatReference uses the same representation as
// LocalReference with the last couple of bytes removed.
//
// FlatReference is sufficient for the read caching and local storage
// backends, which are oblivious of references between objects.
type FlatReference struct {
	rawReference [35]byte
}

// MustNewSHA256V1LocalReference creates a flat reference that uses
// reference format SHA256_V1. This function can be used as part of
// tests.
func MustNewSHA256V1FlatReference(hash string, sizeBytes uint32) (flatReference FlatReference) {
	var rawReference [36]byte
	if n, err := hex.Decode(rawReference[:], []byte(hash)); err != nil {
		panic(err)
	} else if n != 32 {
		panic("Wrong hash length")
	}
	binary.LittleEndian.PutUint32(rawReference[32:], sizeBytes)
	copy(flatReference.rawReference[:], rawReference[:])
	return
}

func (r FlatReference) GetLocalReference() (localReference LocalReference) {
	copy(localReference.rawReference[:], r.rawReference[:])
	return
}

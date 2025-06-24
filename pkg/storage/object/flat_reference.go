package object

import (
	"encoding/binary"
	"encoding/hex"
)

// FlatReference is a simplified version of LocalReference that assumes
// that the height and degree of an object is zero. This is sufficient
// for the read caching backend, which is oblivious of references
// between objects.
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

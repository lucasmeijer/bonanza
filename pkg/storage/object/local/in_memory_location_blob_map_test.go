package local_test

import (
	"math/rand"
	"testing"

	object_local "bonanza.build/pkg/storage/object/local"

	"github.com/stretchr/testify/require"
)

func TestInMemoryLocationBlobMap(t *testing.T) {
	const maximumBlobSizeBytes = 1024
	locationBlobMap := object_local.NewInMemoryLocationBlobMap(maximumBlobSizeBytes)

	var blob [maximumBlobSizeBytes]byte
	_, err := rand.Read(blob[:])
	require.NoError(t, err)

	expectedLocation := uint64(0)
	for i := 1; i <= maximumBlobSizeBytes; i++ {
		// Write a blob.
		actualLocation, err := locationBlobMap.Put(blob[:i])
		require.NoError(t, err)
		require.Equal(t, expectedLocation, actualLocation)
		expectedLocation += uint64(i)

		// Reading a blob at the same location should give the
		// same data back.
		actualBlob, err := locationBlobMap.Get(actualLocation, i)
		require.NoError(t, err)
		require.Equal(t, blob[:i], actualBlob)
	}
}

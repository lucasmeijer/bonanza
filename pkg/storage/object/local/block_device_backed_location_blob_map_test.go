package local_test

import (
	"math/rand"
	"testing"

	"bonanza.build/pkg/storage/object/local"

	"github.com/stretchr/testify/require"

	"go.uber.org/mock/gomock"
)

func TestBlockDeviceBackedLocationBlobMap(t *testing.T) {
	ctrl := gomock.NewController(t)

	blockDevice := NewMockBlockDevice(ctrl)
	locationBlobMap := local.NewBlockDeviceBackedLocationBlobMap(
		blockDevice,
		/* sectorSizeBytes = */ 16,
		/* sectorCount = */ 10,
		/* initialLocation = */ 0,
	)

	t.Run("Get", func(t *testing.T) {
		// The block device should behave as a ring buffer. This
		// means that most objects can be obtained by issuing a
		// single read. However, there may always be an object
		// that wraps around the end of the block device. For
		// this object we need to issue two read operations.
		t.Run("Start", func(t *testing.T) {
			blockDevice.EXPECT().ReadAt(gomock.Len(10), int64(0)).
				DoAndReturn(func(p []byte, offset int64) (int, error) {
					return copy(p, "HelloWorld"), nil
				})

			data, err := locationBlobMap.Get(0x0443fbc06fec3880, 10)
			require.NoError(t, err)
			require.Equal(t, []byte("HelloWorld"), data)
		})

		t.Run("Middle", func(t *testing.T) {
			blockDevice.EXPECT().ReadAt(gomock.Len(10), int64(115)).
				DoAndReturn(func(p []byte, offset int64) (int, error) {
					return copy(p, "Buildbarn!"), nil
				})

			data, err := locationBlobMap.Get(0x76d035a6061a7d53, 10)
			require.NoError(t, err)
			require.Equal(t, []byte("Buildbarn!"), data)
		})

		t.Run("End", func(t *testing.T) {
			blockDevice.EXPECT().ReadAt(gomock.Len(10), int64(150)).
				DoAndReturn(func(p []byte, offset int64) (int, error) {
					return copy(p, "Bonanza!!!"), nil
				})

			data, err := locationBlobMap.Get(0xb92b439023d084f6, 10)
			require.NoError(t, err)
			require.Equal(t, []byte("Bonanza!!!"), data)
		})

		t.Run("WrapAround", func(t *testing.T) {
			blockDevice.EXPECT().ReadAt(gomock.Len(2), int64(158)).
				DoAndReturn(func(p []byte, offset int64) (int, error) {
					return copy(p, "He"), nil
				})
			blockDevice.EXPECT().ReadAt(gomock.Len(8), int64(0)).
				DoAndReturn(func(p []byte, offset int64) (int, error) {
					return copy(p, "lloWorld"), nil
				})

			data, err := locationBlobMap.Get(0x4425786b048827de, 10)
			require.NoError(t, err)
			require.Equal(t, []byte("HelloWorld"), data)
		})
	})

	t.Run("Put", func(t *testing.T) {
		const maximumBlobSizeBytes = 16 * 10
		var blob [maximumBlobSizeBytes]byte
		_, err := rand.Read(blob[:])
		require.NoError(t, err)

		var blockDeviceContents [maximumBlobSizeBytes]byte
		blockDevice.EXPECT().ReadAt(gomock.Any(), gomock.Any()).
			DoAndReturn(func(p []byte, offset int64) (int, error) {
				require.True(t, len(p) > 0)
				return copy(p, blockDeviceContents[offset:]), nil
			}).
			AnyTimes()
		blockDevice.EXPECT().WriteAt(gomock.Any(), gomock.Any()).
			DoAndReturn(func(p []byte, offset int64) (int, error) {
				require.True(t, len(p) > 0 && len(p)%16 == 0)
				require.True(t, offset%16 == 0)
				return copy(blockDeviceContents[offset:], p), nil
			}).
			AnyTimes()

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
	})
}

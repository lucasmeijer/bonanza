package local

import (
	"bonanza.build/pkg/ds/lossymap"
	"bonanza.build/pkg/storage/object"
)

// The reference-location map that is used by this storage backend is
// implemented as a lossy map. The type aliases below are provided for
// readability.
type (
	ReferenceLocationRecordKey   = lossymap.RecordKey[object.FlatReference]
	ReferenceLocationRecord      = lossymap.Record[object.FlatReference, uint64]
	ReferenceLocationRecordArray = lossymap.RecordArray[object.FlatReference, uint64, EpochIDResolver]
)

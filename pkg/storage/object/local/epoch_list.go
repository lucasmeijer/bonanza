package local

// EpochList is used by the local object store to manage the creation
// and lifetime of epochs. Epochs are assigned to reference-location map
// entries and act as a way of ensuring consistency between the
// reference-location map and location-blob map, even if unclean
// shutdowns are performed.
type EpochList interface {
	EpochIDResolver

	// Indicate that a write of an object to the location-blob map
	// ending at a given location has completed, and that a
	// reference-location map entry is about to be written.
	FinalizeWriteUpToLocation(location uint64) error

	// Indicate that an object ending at a given location was read
	// from the location-blob map, but that its contents were
	// invalid. This means that either data corruption has occurred,
	// or that the object got overwritten while being read.
	DiscardUpToLocation(location uint64)
}

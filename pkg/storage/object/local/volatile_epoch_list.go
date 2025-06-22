package local

import (
	"github.com/buildbarn/bb-storage/pkg/random"
)

type volatileEpochList struct {
	maximumLocationSpan   uint64
	randomNumberGenerator random.SingleThreadedGenerator

	epochID    uint32
	epochState EpochState
}

func (el *volatileEpochList) reinitialize() {
	el.epochID = el.randomNumberGenerator.Uint32()
	el.epochState = EpochState{
		HashSeed: el.randomNumberGenerator.Uint64(),
	}
}

// NewVolatileEpochList creates a simple EpochList that does not support
// any persistency. This is sufficient for setups where losing data upon
// restart is acceptable (e.g., worker level caches).
func NewVolatileEpochList(maximumLocationSpan uint64, randomNumberGenerator random.SingleThreadedGenerator) EpochList {
	el := &volatileEpochList{
		maximumLocationSpan:   maximumLocationSpan,
		randomNumberGenerator: randomNumberGenerator,
	}
	el.reinitialize()
	return el
}

func (el *volatileEpochList) FinalizeWriteUpToLocation(location uint64) error {
	if int64(location-el.epochState.MaximumLocation) > 0 {
		// Under the rare circumstance that we've written more
		// than 2^64 bytes of data, LocationBlobMap starts
		// allocating at zero again. This causes full data loss.
		// If that happens, reinitialize the EpochList, so that
		// all reference-location map entries are invalidated.
		if location < el.epochState.MaximumLocation {
			el.reinitialize()
		}
		el.epochState.MaximumLocation = location

		// Writing the data will cause old objects to get
		// overwritten. Progress the minimum location.
		if el.epochState.MaximumLocation-el.epochState.MinimumLocation > el.maximumLocationSpan {
			el.epochState.MinimumLocation = el.epochState.MaximumLocation - el.maximumLocationSpan
		}
	}
	return nil
}

func (el *volatileEpochList) DiscardUpToLocation(location uint64) {
	if el.epochState.MinimumLocation < location {
		el.epochState.MinimumLocation = location
		if el.epochState.MaximumLocation < el.epochState.MinimumLocation {
			el.epochState.MaximumLocation = el.epochState.MinimumLocation
		}
	}
}

func (el *volatileEpochList) GetEpochStateForEpochID(epochID uint32) (EpochState, bool) {
	if epochID != el.epochID {
		return EpochState{}, false
	}
	return el.epochState, true
}

func (el *volatileEpochList) GetCurrentEpochState() (EpochState, uint32) {
	return el.epochState, el.epochID
}

package core

// NoopReferenceMetadata is a trivial implementation of
// ReferenceMetadata that does not capture anything. This can be used if
// the goal is to simply compute the root node of the Merkle tree,
// discarding any children that were created in the process.
type NoopReferenceMetadata struct{}

// Discard all resources owned by this instance of
// NoopReferenceMetadata. This method is merely provided to satisfy the
// ReferenceMetadata interface.
func (NoopReferenceMetadata) Discard() {}

type discardingCreatedObjectCapturer struct{}

// DiscardingCreatedObjectCapturer is an implementation of
// CreatedObjectCapturer that discards any created objects. This can be
// used if the goal is to simply compute the root node of the Merkle
// tree, discarding any children that were created in the process.
var DiscardingCreatedObjectCapturer CreatedObjectCapturer[NoopReferenceMetadata] = discardingCreatedObjectCapturer{}

func (discardingCreatedObjectCapturer) CaptureCreatedObject(CreatedObject[NoopReferenceMetadata]) NoopReferenceMetadata {
	return NoopReferenceMetadata{}
}

package filesystem

import (
	model_core "bonanza.build/pkg/model/core"
	"bonanza.build/pkg/storage/object"
)

// FileMerkleTreeCapturer is provided by callers of CreateFileMerkleTree
// to provide logic for how the resulting Merkle tree of the file should
// be captured.
//
// A no-op implementation can be used by the caller to simply compute a
// reference of the file. An implementation that actually captures the
// provided contents can be used to prepare a Merkle tree for uploading.
//
// The methods below return metadata. The metadata for the root object
// will be returned by CreateFileMerkleTree.
type FileMerkleTreeCapturer[TMetadata any] interface {
	CaptureChunk(contents *object.Contents) TMetadata
	CaptureFileContentsList(createdObject model_core.CreatedObject[TMetadata]) TMetadata
}

type chunkDiscardingFileMerkleTreeCapturer struct{}

// ChunkDiscardingFileMerkleTreeCapturer is an implementation of
// FileMerkleTreeCapturer that only preserves the FileContents messages
// of the Merkle tree. This can be of use when incrementally replicating
// the contents of a file. In those cases it's wasteful to store the
// full contents of a file in memory.
var ChunkDiscardingFileMerkleTreeCapturer FileMerkleTreeCapturer[model_core.CreatedObjectTree] = chunkDiscardingFileMerkleTreeCapturer{}

func (chunkDiscardingFileMerkleTreeCapturer) CaptureChunk(contents *object.Contents) model_core.CreatedObjectTree {
	return model_core.CreatedObjectTree{}
}

func (chunkDiscardingFileMerkleTreeCapturer) CaptureFileContentsList(createdObject model_core.CreatedObject[model_core.CreatedObjectTree]) model_core.CreatedObjectTree {
	o := model_core.CreatedObjectTree{
		Contents: createdObject.Contents,
	}
	if createdObject.GetHeight() > 1 {
		o.Metadata = createdObject.Metadata
	}
	return o
}

type simpleFileMerkleTreeCapturer[TMetadata any] struct {
	capturer model_core.CreatedObjectCapturer[TMetadata]
}

// NewSimpleFileMerkleTreeCapturer creates a FileMerkleTreeCapturer that
// assumes that chunks and file contents lists need to be captured the
// same way.
func NewSimpleFileMerkleTreeCapturer[TMetadata any](capturer model_core.CreatedObjectCapturer[TMetadata]) FileMerkleTreeCapturer[TMetadata] {
	return simpleFileMerkleTreeCapturer[TMetadata]{
		capturer: capturer,
	}
}

func (c simpleFileMerkleTreeCapturer[TMetadata]) CaptureChunk(contents *object.Contents) TMetadata {
	return c.capturer.CaptureCreatedObject(model_core.CreatedObject[TMetadata]{Contents: contents})
}

func (c simpleFileMerkleTreeCapturer[TMetadata]) CaptureFileContentsList(createdObject model_core.CreatedObject[TMetadata]) TMetadata {
	return c.capturer.CaptureCreatedObject(createdObject)
}

type FileMerkleTreeCapturerForTesting FileMerkleTreeCapturer[model_core.ReferenceMetadata]

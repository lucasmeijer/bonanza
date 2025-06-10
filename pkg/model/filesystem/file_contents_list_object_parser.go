package filesystem

import (
	"math"
	"math/bits"

	"github.com/buildbarn/bb-storage/pkg/util"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/parser"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	model_filesystem_pb "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FileContentsEntry contains the properties of a part of a concatenated
// file. Note that the Reference field is only set when EndBytes is
// non-zero.
type FileContentsEntry[TReference any] struct {
	EndBytes  uint64
	Reference model_core.Decodable[TReference]
}

func flattenFileContentsReference[TReference object.BasicReference](fileContents model_core.Message[*model_filesystem_pb.FileContents, TReference]) (model_core.Decodable[TReference], error) {
	var bad model_core.Decodable[TReference]
	switch level := fileContents.Message.Level.(type) {
	case *model_filesystem_pb.FileContents_ChunkReference:
		reference, err := model_core.FlattenDecodableReference(model_core.Nested(fileContents, level.ChunkReference))
		if err != nil {
			return bad, err
		}
		if reference.Value.GetHeight() != 0 {
			return bad, status.Error(codes.InvalidArgument, "Chunk reference must have height 0")
		}
		return reference, nil
	case *model_filesystem_pb.FileContents_FileContentsListReference:
		reference, err := model_core.FlattenDecodableReference(model_core.Nested(fileContents, level.FileContentsListReference))
		if err != nil {
			return bad, err
		}
		if reference.Value.GetHeight() == 0 {
			return bad, status.Error(codes.InvalidArgument, "File contents list reference cannot have height 0")
		}
		return reference, nil
	default:
		return bad, status.Error(codes.InvalidArgument, "Unknown reference type")
	}
}

// NewFileContentsEntryFromProto constructs a FileContentsEntry based on
// the contents of a single FileContents Protobuf message, refering to
// the file as a whole.
func NewFileContentsEntryFromProto[TReference object.BasicReference](fileContents model_core.Message[*model_filesystem_pb.FileContents, TReference]) (FileContentsEntry[TReference], error) {
	if fileContents.Message == nil {
		// File is empty, meaning that it is not backed by any
		// object. Leave the reference unset.
		return FileContentsEntry[TReference]{EndBytes: 0}, nil
	}

	reference, err := flattenFileContentsReference(fileContents)
	if err != nil {
		return FileContentsEntry[TReference]{}, err
	}
	return FileContentsEntry[TReference]{
		EndBytes:  fileContents.Message.TotalSizeBytes,
		Reference: reference,
	}, nil
}

// FileContentsEntryToProto converts a FileContentsEntry back to a
// Protobuf message.
//
// TODO: Should this function take a model_core.ExistingObjectCapturer?
func FileContentsEntryToProto[TReference object.BasicReference](
	entry *FileContentsEntry[TReference],
) model_core.PatchedMessage[*model_filesystem_pb.FileContents, dag.ObjectContentsWalker] {
	if entry.EndBytes == 0 {
		// Empty file is encoded as a nil message.
		return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker]((*model_filesystem_pb.FileContents)(nil))
	}

	if entry.Reference.Value.GetHeight() > 0 {
		// Large file.
		return model_core.BuildPatchedMessage(func(patcher *model_core.ReferenceMessagePatcher[dag.ObjectContentsWalker]) *model_filesystem_pb.FileContents {
			return &model_filesystem_pb.FileContents{
				Level: &model_filesystem_pb.FileContents_FileContentsListReference{
					FileContentsListReference: &model_core_pb.DecodableReference{
						Reference: patcher.AddReference(
							entry.Reference.Value.GetLocalReference(),
							dag.ExistingObjectContentsWalker,
						),
						DecodingParameters: entry.Reference.GetDecodingParameters(),
					},
				},
				TotalSizeBytes: entry.EndBytes,
			}
		})
	}

	// Small file.
	return model_core.BuildPatchedMessage(func(patcher *model_core.ReferenceMessagePatcher[dag.ObjectContentsWalker]) *model_filesystem_pb.FileContents {
		return &model_filesystem_pb.FileContents{
			Level: &model_filesystem_pb.FileContents_ChunkReference{
				ChunkReference: &model_core_pb.DecodableReference{
					Reference: patcher.AddReference(
						entry.Reference.Value.GetLocalReference(),
						dag.ExistingObjectContentsWalker,
					),
					DecodingParameters: entry.Reference.GetDecodingParameters(),
				},
			},
			TotalSizeBytes: entry.EndBytes,
		}
	})
}

// FileContentsList contains the properties of parts of a concatenated
// file. Parts are stored in the order in which they should be
// concatenated, with EndBytes increasing.
type FileContentsList[TReference any] []FileContentsEntry[TReference]

type fileContentsListObjectParser[TReference object.BasicReference] struct{}

// NewFileContentsListObjectParser creates an ObjectParser that is
// capable of parsing FileContentsList messages, turning them into a
// list of entries that can be processed by FileContentsIterator.
func NewFileContentsListObjectParser[TReference object.BasicReference]() parser.ObjectParser[TReference, FileContentsList[TReference]] {
	return &fileContentsListObjectParser[TReference]{}
}

func (p *fileContentsListObjectParser[TReference]) ParseObject(in model_core.Message[[]byte, TReference], decodingParameters []byte) (FileContentsList[TReference], int, error) {
	l, sizeBytes, err := model_parser.NewMessageListObjectParser[TReference, model_filesystem_pb.FileContents]().
		ParseObject(in, decodingParameters)
	if err != nil {
		return nil, 0, err
	}
	if len(l.Message) < 2 {
		return nil, 0, status.Error(codes.InvalidArgument, "File contents list contains fewer than two parts")
	}

	var endBytes uint64
	fileContentsList := make(FileContentsList[TReference], 0, len(l.Message))
	for i, part := range l.Message {
		// Convert 'total_size_bytes' to a cumulative value, to
		// allow FileContentsIterator to perform binary searching.
		if part.TotalSizeBytes < 1 {
			return nil, 0, status.Errorf(codes.InvalidArgument, "Part at index %d does not contain any data", i)
		}
		var carryOut uint64
		endBytes, carryOut = bits.Add64(endBytes, part.TotalSizeBytes, 0)
		if carryOut > 0 {
			return nil, 0, status.Errorf(codes.InvalidArgument, "Combined size of all parts exceeds maximum file size of %d bytes", uint64(math.MaxUint64))
		}

		partReference, err := flattenFileContentsReference(model_core.Nested(l, part))
		if err != nil {
			return nil, 0, util.StatusWrapf(err, "Invalid reference for part at index %d", i)
		}

		fileContentsList = append(fileContentsList, FileContentsEntry[TReference]{
			EndBytes:  endBytes,
			Reference: partReference,
		})
	}
	return fileContentsList, sizeBytes, nil
}

func (p *fileContentsListObjectParser[TReference]) GetDecodingParametersSizeBytes() int {
	return 0
}

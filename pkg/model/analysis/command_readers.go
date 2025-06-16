package analysis

import (
	"context"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_command_pb "github.com/buildbarn/bonanza/pkg/proto/model/command"
)

// CommandReaders contains ParsedObjectReaders that can be used to
// follow references to objects that are encoded using the command
// encoders that are part of the BuildSpecification.
type CommandReaders[TReference any] struct {
	PathPatternChildren model_parser.ParsedObjectReader[model_core.Decodable[TReference], model_core.Message[*model_command_pb.PathPattern_Children, TReference]]
}

func (c *baseComputer[TReference, TMetadata]) ComputeCommandReadersValue(ctx context.Context, key *model_analysis_pb.CommandReaders_Key, e CommandReadersEnvironment[TReference, TMetadata]) (*CommandReaders[TReference], error) {
	commandEncoder, gotCommandEncoder := e.GetCommandEncoderObjectValue(&model_analysis_pb.CommandEncoderObject_Key{})
	if !gotCommandEncoder {
		return nil, evaluation.ErrMissingDependency
	}
	encodedObjectParser := model_parser.NewEncodedObjectParser[TReference](commandEncoder)
	return &CommandReaders[TReference]{
		PathPatternChildren: model_parser.LookupParsedObjectReader(
			c.parsedObjectPoolIngester,
			model_parser.NewChainedObjectParser(
				encodedObjectParser,
				model_parser.NewProtoObjectParser[TReference, model_command_pb.PathPattern_Children](),
			),
		),
	}, nil
}

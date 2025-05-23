package analysis

import (
	"context"
	"errors"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"

	"go.starlark.net/starlark"
)

func (c *baseComputer[TReference, TMetadata]) ComputeCtxWrapperValue(ctx context.Context, key *model_analysis_pb.CtxWrapper_Key, e CtxWrapperEnvironment[TReference, TMetadata]) (starlark.Value, error) {
	buildSpecification := e.GetBuildSpecificationValue(&model_analysis_pb.BuildSpecification_Key{})
	if !buildSpecification.IsSet() {
		return nil, evaluation.ErrMissingDependency
	}

	identifier := buildSpecification.Message.BuildSpecification.GetCtxWrapperIdentifier()
	if identifier == "" {
		// No wrapper configured. Return an identity function
		// that leaves ctx in its original form.
		return starlark.NewBuiltin("ctx_wrapper", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return args[0], nil
		}), nil
	}
	ctxWrapperValue := e.GetCompiledBzlFileGlobalValue(&model_analysis_pb.CompiledBzlFileGlobal_Key{
		Identifier: identifier,
	})
	if !ctxWrapperValue.IsSet() {
		return nil, evaluation.ErrMissingDependency
	}

	ctxWrapperFunction, ok := ctxWrapperValue.Message.Global.GetKind().(*model_starlark_pb.Value_Function)
	if !ok {
		return nil, errors.New("global value is not a function")
	}
	return model_starlark.NewNamedFunction(
		model_starlark.NewProtoNamedFunctionDefinition[TReference, TMetadata](
			model_core.Nested(ctxWrapperValue, ctxWrapperFunction.Function),
		),
	), nil
}

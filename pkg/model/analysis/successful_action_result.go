package analysis

import (
	"context"
	"fmt"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
)

func (c *baseComputer[TReference, TMetadata]) ComputeSuccessfulActionResultValue(ctx context.Context, key model_core.Message[*model_analysis_pb.SuccessfulActionResult_Key, TReference], e SuccessfulActionResultEnvironment[TReference, TMetadata]) (PatchedSuccessfulActionResultValue, error) {
	patchedAction := model_core.Patch(e, model_core.Nested(key, key.Message.Action))
	actionResult := e.GetActionResultValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.ActionResult_Key{
				Action: patchedAction.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedAction.Patcher),
		),
	)
	if !actionResult.IsSet() {
		return PatchedSuccessfulActionResultValue{}, evaluation.ErrMissingDependency
	}

	if exitCode := actionResult.Message.ExitCode; exitCode != 0 {
		return PatchedSuccessfulActionResultValue{}, fmt.Errorf("action completed with non-zero exit code %d", exitCode)
	}

	patchedOutputsReference := model_core.Patch(e, model_core.Nested(actionResult, actionResult.Message.OutputsReference))
	return model_core.NewPatchedMessage(
		&model_analysis_pb.SuccessfulActionResult_Value{
			OutputsReference: patchedOutputsReference.Message,
		},
		model_core.MapReferenceMetadataToWalkers(patchedOutputsReference.Patcher),
	), nil
}

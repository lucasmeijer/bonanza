package analysis

import (
	"bytes"
	"context"
	"errors"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/btree"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
)

func (c *baseComputer[TReference, TMetadata]) ComputeTargetActionResultValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetActionResult_Key, TReference], e TargetActionResultEnvironment[TReference, TMetadata]) (PatchedTargetActionResultValue, error) {
	patchedConfigurationReference := model_core.Patch(e, model_core.Nested(key, key.Message.ConfigurationReference))
	configuredTarget := e.GetConfiguredTargetValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.ConfiguredTarget_Key{
				Label:                  key.Message.Label,
				ConfigurationReference: patchedConfigurationReference.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedConfigurationReference.Patcher),
		),
	)
	if !configuredTarget.IsSet() {
		return PatchedTargetActionResultValue{}, evaluation.ErrMissingDependency
	}

	actionID := key.Message.ActionId
	action, err := btree.Find(
		ctx,
		c.configuredTargetActionReader,
		model_core.Nested(configuredTarget, configuredTarget.Message.Actions),
		func(entry *model_analysis_pb.ConfiguredTarget_Value_Action) (int, *model_core_pb.Reference) {
			switch level := entry.Level.(type) {
			case *model_analysis_pb.ConfiguredTarget_Value_Action_Leaf_:
				return bytes.Compare(actionID, level.Leaf.Id), nil
			case *model_analysis_pb.ConfiguredTarget_Value_Action_Parent_:
				return bytes.Compare(actionID, level.Parent.FirstId), level.Parent.Reference
			default:
				return 0, nil
			}
		},
	)
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}
	if !action.IsSet() {
		return PatchedTargetActionResultValue{}, errors.New("target does not yield an action with the provided identifier")
	}
	actionLevel, ok := action.Message.Level.(*model_analysis_pb.ConfiguredTarget_Value_Action_Leaf_)
	if !ok {
		return PatchedTargetActionResultValue{}, errors.New("action is not a leaf")
	}

	_, err = getRootForFiles(e, model_core.Nested(action, actionLevel.Leaf.Inputs))
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}

	return PatchedTargetActionResultValue{}, errors.New("TODO: Invoke action!")
}

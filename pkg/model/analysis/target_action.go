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

func (c *baseComputer[TReference, TMetadata]) ComputeTargetActionValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetAction_Key, TReference], e TargetActionEnvironment[TReference, TMetadata]) (PatchedTargetActionValue, error) {
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
		return PatchedTargetActionValue{}, evaluation.ErrMissingDependency
	}

	actionID := key.Message.ActionId
	action, err := btree.Find(
		ctx,
		c.configuredTargetActionReader,
		model_core.Nested(configuredTarget, configuredTarget.Message.Actions),
		func(entry model_core.Message[*model_analysis_pb.ConfiguredTarget_Value_Action, TReference]) (int, *model_core_pb.DecodableReference) {
			switch level := entry.Message.Level.(type) {
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
		return PatchedTargetActionValue{}, err
	}
	if !action.IsSet() {
		return PatchedTargetActionValue{}, errors.New("target does not yield an action with the provided identifier")
	}
	actionLevel, ok := action.Message.Level.(*model_analysis_pb.ConfiguredTarget_Value_Action_Leaf_)
	if !ok {
		return PatchedTargetActionValue{}, errors.New("action is not a leaf")
	}

	patchedDefinition := model_core.Patch(e, model_core.Nested(action, actionLevel.Leaf.Definition))
	return model_core.NewPatchedMessage(
		&model_analysis_pb.TargetAction_Value{
			Definition: patchedDefinition.Message,
		},
		model_core.MapReferenceMetadataToWalkers(patchedDefinition.Patcher),
	), nil
}

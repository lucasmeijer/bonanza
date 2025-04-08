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
	"github.com/buildbarn/bonanza/pkg/storage/dag"

	"google.golang.org/protobuf/types/known/durationpb"
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
	actionLeaf := actionLevel.Leaf

	_, err = getRootForFiles(e, model_core.Nested(action, actionLeaf.Inputs))
	if err != nil {
		return PatchedTargetActionResultValue{}, err
	}

	patcher := model_core.NewReferenceMessagePatcher[dag.ObjectContentsWalker]()
	actionResult := e.GetActionResultValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.ActionResult_Key{
				// TODO: Provide command and input root!
				// CommandReference:      xyz,
				// TODO: Should we make the execution
				// timeout on build actions configurable?
				// Bazel with REv2 does not set this field
				// for build actions, relying on the cluster
				// to pick a default.
				ExecutionTimeout:   &durationpb.Duration{Seconds: 3600},
				ExitCodeMustBeZero: true,
				// InputRootReference:    xyz,
				PlatformPkixPublicKey: actionLeaf.PlatformPkixPublicKey,
			},
			patcher,
		),
	)
	if !actionResult.IsSet() {
		return PatchedTargetActionResultValue{}, evaluation.ErrMissingDependency
	}

	return PatchedTargetActionResultValue{}, errors.New("TODO: Invoke action!")
}

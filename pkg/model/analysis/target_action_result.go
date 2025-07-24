package analysis

import (
	"context"
	"errors"
	"fmt"

	"bonanza.build/pkg/evaluation"
	model_core "bonanza.build/pkg/model/core"
	model_parser "bonanza.build/pkg/model/parser"
	model_analysis_pb "bonanza.build/pkg/proto/model/analysis"
	model_command_pb "bonanza.build/pkg/proto/model/command"

	"google.golang.org/protobuf/types/known/durationpb"
)

func (c *baseComputer[TReference, TMetadata]) ComputeTargetActionResultValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetActionResult_Key, TReference], e TargetActionResultEnvironment[TReference, TMetadata]) (PatchedTargetActionResultValue, error) {
	id := model_core.Nested(key, key.Message.Id)
	if id.Message == nil {
		return PatchedTargetActionResultValue{}, errors.New("no target action identifier specified")
	}
	patchedID1 := model_core.Patch(e, id)
	action := e.GetTargetActionValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.TargetAction_Key{
				Id: patchedID1.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedID1.Patcher),
		),
	)
	patchedID2 := model_core.Patch(e, id)
	command := e.GetTargetActionCommandValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.TargetActionCommand_Key{
				Id: patchedID2.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedID2.Patcher),
		),
	)
	commandEncoder, gotCommandEncoder := e.GetCommandEncoderObjectValue(&model_analysis_pb.CommandEncoderObject_Key{})
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	patchedID3 := model_core.Patch(e, id)
	inputRoot := e.GetTargetActionInputRootValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.TargetActionInputRoot_Key{
				Id: patchedID3.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedID3.Patcher),
		),
	)
	if !action.IsSet() || !command.IsSet() || !gotCommandEncoder || !gotDirectoryReaders || !inputRoot.IsSet() {
		return PatchedTargetActionResultValue{}, evaluation.ErrMissingDependency
	}

	actionDefinition := action.Message.Definition
	if actionDefinition == nil {
		return PatchedTargetActionResultValue{}, errors.New("action definition missing")
	}

	commandReference := model_core.Patch(e, model_core.Nested(command, command.Message.CommandReference))
	inputRootReference := model_core.Patch(e, model_core.Nested(inputRoot, inputRoot.Message.InputRootReference))
	referenceFormat := c.getReferenceFormat()
	createdAction, err := model_core.MarshalAndEncode(
		model_core.BuildPatchedMessage(func(patcher *model_core.ReferenceMessagePatcher[TMetadata]) model_core.Marshalable {
			patcher.Merge(commandReference.Patcher)
			patcher.Merge(inputRootReference.Patcher)
			return model_core.NewProtoMarshalable(&model_command_pb.Action{
				CommandReference:   commandReference.Message,
				InputRootReference: inputRootReference.Message,
			})
		}),
		referenceFormat,
		commandEncoder,
	)
	if err != nil {
		return PatchedTargetActionResultValue{}, fmt.Errorf("failed to create action: %w", err)
	}

	patcher := model_core.NewReferenceMessagePatcher[TMetadata]()
	actionResult := e.GetSuccessfulActionResultValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.SuccessfulActionResult_Key{
				ExecuteRequest: &model_analysis_pb.ExecuteRequest{
					// TODO: Should we make the execution
					// timeout on build actions configurable?
					// Bazel with REv2 does not set this field
					// for build actions, relying on the cluster
					// to pick a default.
					ExecutionTimeout:      &durationpb.Duration{Seconds: 3600},
					ActionReference:       patcher.CaptureAndAddDecodableReference(createdAction, e),
					PlatformPkixPublicKey: actionDefinition.PlatformPkixPublicKey,
				},
			},
			model_core.MapReferenceMetadataToWalkers(patcher),
		),
	)
	if !actionResult.IsSet() {
		return PatchedTargetActionResultValue{}, evaluation.ErrMissingDependency
	}

	outputs, err := model_parser.MaybeDereference(
		ctx,
		directoryReaders.CommandOutputs,
		model_core.Nested(actionResult, actionResult.Message.OutputsReference),
	)
	if err != nil {
		return PatchedTargetActionResultValue{}, fmt.Errorf("failed to obtain outputs from action result: %w", err)
	}
	outputRoot := model_core.Patch(e, model_core.Nested(outputs, outputs.Message.GetOutputRoot()))
	return model_core.NewPatchedMessage(
		&model_analysis_pb.TargetActionResult_Value{
			OutputRoot: outputRoot.Message,
		},
		model_core.MapReferenceMetadataToWalkers(outputRoot.Patcher),
	), nil
}

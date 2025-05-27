package analysis

import (
	"context"
	"crypto/ecdh"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/btree"
	model_encoding "github.com/buildbarn/bonanza/pkg/model/encoding"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_command_pb "github.com/buildbarn/bonanza/pkg/proto/model/command"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	remoteexecution_pb "github.com/buildbarn/bonanza/pkg/proto/remoteexecution"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/grpc/status"
)

func (c *baseComputer[TReference, TMetadata]) ComputeActionResultValue(ctx context.Context, key model_core.Message[*model_analysis_pb.ActionResult_Key, TReference], e ActionResultEnvironment[TReference, TMetadata]) (PatchedActionResultValue, error) {
	commandEncodersValue := e.GetCommandEncodersValue(&model_analysis_pb.CommandEncoders_Key{})
	if !commandEncodersValue.IsSet() {
		return PatchedActionResultValue{}, evaluation.ErrMissingDependency
	}

	// Compute shared secret for encrypting the action.
	action := model_core.Nested(key, key.Message.Action)
	if action.Message == nil {
		return PatchedActionResultValue{}, errors.New("no action specified")
	}
	platformPublicKey, err := x509.ParsePKIXPublicKey(action.Message.PlatformPkixPublicKey)
	if err != nil {
		return PatchedActionResultValue{}, fmt.Errorf("invalid platform PKIX public key: %w", err)
	}
	platformECDHPublicKey, ok := platformPublicKey.(*ecdh.PublicKey)
	if !ok {
		return PatchedActionResultValue{}, errors.New("platform PKIX public key is not an ECDH public key")
	}

	// Use the reference of the Command message as the stable
	// fingerprint of the action, which the scheduler can use to
	// keep track of performance characteristics. Compute a hash to
	// masquerade the actual Command reference.
	commandReference, err := model_core.FlattenDecodableReference(model_core.Nested(action, action.Message.CommandReference))
	if err != nil {
		return PatchedActionResultValue{}, fmt.Errorf("invalid command reference: %w", err)
	}
	commandReferenceSHA256 := sha256.Sum256(commandReference.Value.GetRawReference())

	inputRootReference, err := model_core.FlattenDecodableReference(model_core.Nested(action, action.Message.InputRootReference))
	if err != nil {
		return PatchedActionResultValue{}, fmt.Errorf("invalid input root reference: %w", err)
	}

	var completionEvent model_command_pb.Result
	var errExecution error
	for range c.executionClient.RunAction(
		ctx,
		platformECDHPublicKey,
		&model_command_pb.Action{
			Namespace:          c.executionNamespace,
			CommandEncoders:    commandEncodersValue.Message.CommandEncoders,
			CommandReference:   model_core.DecodableLocalReferenceToWeakProto(commandReference),
			InputRootReference: model_core.DecodableLocalReferenceToWeakProto(inputRootReference),
		},
		&remoteexecution_pb.Action_AdditionalData{
			StableFingerprint: commandReferenceSHA256[:],
			ExecutionTimeout:  action.Message.ExecutionTimeout,
		},
		&completionEvent,
		&errExecution,
	) {
		// TODO: Capture and propagate execution events?
	}
	if errExecution != nil {
		return PatchedActionResultValue{}, errExecution
	}

	if err := status.ErrorProto(completionEvent.Status); err != nil {
		return PatchedActionResultValue{}, err
	}

	result := &model_analysis_pb.ActionResult_Value{
		ExitCode: completionEvent.ExitCode,
	}
	patcher := model_core.NewReferenceMessagePatcher[dag.ObjectContentsWalker]()
	if completionEvent.OutputsReference != nil {
		outputsReference, err := model_core.NewDecodableLocalReferenceFromWeakProto(c.getReferenceFormat(), completionEvent.OutputsReference)
		if err != nil {
			return PatchedActionResultValue{}, fmt.Errorf("invalid outputs reference: %w", err)
		}
		result.OutputsReference = &model_core_pb.DecodableReference{
			Reference:          patcher.AddReference(outputsReference.Value, dag.ExistingObjectContentsWalker),
			DecodingParameters: outputsReference.GetDecodingParameters(),
		}
	}
	return model_core.NewPatchedMessage(result, patcher), nil
}

func convertDictToEnvironmentVariableList[TMetadata model_core.ReferenceMetadata](
	environment map[string]string,
	commandEncoder model_encoding.BinaryEncoder,
	referenceFormat object.ReferenceFormat,
	capturer model_core.CreatedObjectCapturer[TMetadata],
) (model_core.PatchedMessage[[]*model_command_pb.EnvironmentVariableList_Element, TMetadata], error) {
	environmentVariablesBuilder := btree.NewSplitProllyBuilder(
		1<<16,
		1<<18,
		btree.NewObjectCreatingNodeMerger(
			commandEncoder,
			referenceFormat,
			/* parentNodeComputer = */ func(createdObject model_core.Decodable[model_core.CreatedObject[TMetadata]], childNodes []*model_command_pb.EnvironmentVariableList_Element) (model_core.PatchedMessage[*model_command_pb.EnvironmentVariableList_Element, TMetadata], error) {
				patcher := model_core.NewReferenceMessagePatcher[TMetadata]()
				return model_core.NewPatchedMessage(
					&model_command_pb.EnvironmentVariableList_Element{
						Level: &model_command_pb.EnvironmentVariableList_Element_Parent{
							Parent: patcher.CaptureAndAddDecodableReference(createdObject, capturer),
						},
					},
					patcher,
				), nil
			},
		),
	)
	for _, name := range slices.Sorted(maps.Keys(environment)) {
		if err := environmentVariablesBuilder.PushChild(
			model_core.NewSimplePatchedMessage[TMetadata](&model_command_pb.EnvironmentVariableList_Element{
				Level: &model_command_pb.EnvironmentVariableList_Element_Leaf_{
					Leaf: &model_command_pb.EnvironmentVariableList_Element_Leaf{
						Name:  name,
						Value: environment[name],
					},
				},
			}),
		); err != nil {
			return model_core.PatchedMessage[[]*model_command_pb.EnvironmentVariableList_Element, TMetadata]{}, err
		}
	}
	return environmentVariablesBuilder.FinalizeList()
}

package analysis

import (
	"context"
	"errors"

	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_filesystem "github.com/buildbarn/bonanza/pkg/model/filesystem"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

func (c *baseComputer[TReference, TMetadata]) ComputeTargetActionInputRootValue(ctx context.Context, key model_core.Message[*model_analysis_pb.TargetActionInputRoot_Key, TReference], e TargetActionInputRootEnvironment[TReference, TMetadata]) (PatchedTargetActionInputRootValue, error) {
	id := model_core.Nested(key, key.Message.Id)
	if id.Message == nil {
		return PatchedTargetActionInputRootValue{}, errors.New("no target action identifier specified")
	}

	patchedID := model_core.Patch(e, id)
	action := e.GetTargetActionValue(
		model_core.NewPatchedMessage(
			&model_analysis_pb.TargetAction_Key{
				Id: patchedID.Message,
			},
			model_core.MapReferenceMetadataToWalkers(patchedID.Patcher),
		),
	)
	directoryCreationParameters, gotDirectoryCreationParameters := e.GetDirectoryCreationParametersObjectValue(&model_analysis_pb.DirectoryCreationParametersObject_Key{})
	directoryReaders, gotDirectoryReaders := e.GetDirectoryReadersValue(&model_analysis_pb.DirectoryReaders_Key{})
	fileCreationParametersMessage := e.GetFileCreationParametersValue(&model_analysis_pb.FileCreationParameters_Key{})
	if !action.IsSet() ||
		!gotDirectoryCreationParameters ||
		!gotDirectoryReaders ||
		!fileCreationParametersMessage.IsSet() {
		return PatchedTargetActionInputRootValue{}, evaluation.ErrMissingDependency
	}

	actionDefinition := action.Message.Definition
	if actionDefinition == nil {
		return PatchedTargetActionInputRootValue{}, errors.New("action definition missing")
	}

	var rootDirectory changeTrackingDirectory[TReference, TMetadata]
	loadOptions := &changeTrackingDirectoryLoadOptions[TReference]{
		context:         ctx,
		directoryReader: directoryReaders.Directory,
		leavesReader:    directoryReaders.Leaves,
	}
	if err := addFilesToChangeTrackingDirectory(
		e,
		model_core.Nested(action, actionDefinition.Inputs),
		&rootDirectory,
		loadOptions,
	); err != nil {
		return PatchedTargetActionInputRootValue{}, err
	}
	// TODO: We need to add runfiles for the tools!
	if err := addFilesToChangeTrackingDirectory(
		e,
		model_core.Nested(action, actionDefinition.Tools),
		&rootDirectory,
		loadOptions,
	); err != nil {
		return PatchedTargetActionInputRootValue{}, err
	}

	group, groupCtx := errgroup.WithContext(ctx)
	var createdRootDirectory model_filesystem.CreatedDirectory[TMetadata]
	group.Go(func() error {
		return model_filesystem.CreateDirectoryMerkleTree[TMetadata, TMetadata](
			groupCtx,
			semaphore.NewWeighted(1),
			group,
			directoryCreationParameters,
			&capturableChangeTrackingDirectory[TReference, TMetadata]{
				options: &capturableChangeTrackingDirectoryOptions[TReference, TMetadata]{
					context:         ctx,
					directoryReader: directoryReaders.Directory,
					objectCapturer:  e,
				},
				directory: &rootDirectory,
			},
			model_filesystem.NewSimpleDirectoryMerkleTreeCapturer[TMetadata](e),
			&createdRootDirectory,
		)
	})
	if err := group.Wait(); err != nil {
		return PatchedTargetActionInputRootValue{}, err
	}

	rootDirectoryObject, err := model_core.MarshalAndEncodePatchedMessage(
		createdRootDirectory.Message,
		c.getReferenceFormat(),
		directoryCreationParameters.GetEncoder(),
	)
	if err != nil {
		return PatchedTargetActionInputRootValue{}, err
	}

	patcher := model_core.NewReferenceMessagePatcher[TMetadata]()
	return model_core.NewPatchedMessage(
		&model_analysis_pb.TargetActionInputRoot_Value{
			InputRootReference: patcher.CaptureAndAddDecodableReference(rootDirectoryObject, e),
		},
		model_core.MapReferenceMetadataToWalkers(patcher),
	), nil
}

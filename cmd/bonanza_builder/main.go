package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	re_filesystem "github.com/buildbarn/bb-remote-execution/pkg/filesystem"
	"github.com/buildbarn/bb-storage/pkg/clock"
	"github.com/buildbarn/bb-storage/pkg/filesystem"
	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bb-storage/pkg/global"
	bb_http "github.com/buildbarn/bb-storage/pkg/http"
	"github.com/buildbarn/bb-storage/pkg/program"
	"github.com/buildbarn/bb-storage/pkg/random"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/buildbarn/bonanza/pkg/evaluation"
	model_analysis "github.com/buildbarn/bonanza/pkg/model/analysis"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/btree"
	"github.com/buildbarn/bonanza/pkg/model/encoding"
	model_encoding "github.com/buildbarn/bonanza/pkg/model/encoding"
	model_parser "github.com/buildbarn/bonanza/pkg/model/parser"
	model_starlark "github.com/buildbarn/bonanza/pkg/model/starlark"
	"github.com/buildbarn/bonanza/pkg/proto/configuration/bonanza_builder"
	model_analysis_pb "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	model_build_pb "github.com/buildbarn/bonanza/pkg/proto/model/build"
	model_command_pb "github.com/buildbarn/bonanza/pkg/proto/model/command"
	model_core_pb "github.com/buildbarn/bonanza/pkg/proto/model/core"
	model_evaluation_pb "github.com/buildbarn/bonanza/pkg/proto/model/evaluation"
	remoteexecution_pb "github.com/buildbarn/bonanza/pkg/proto/remoteexecution"
	remoteworker_pb "github.com/buildbarn/bonanza/pkg/proto/remoteworker"
	dag_pb "github.com/buildbarn/bonanza/pkg/proto/storage/dag"
	object_pb "github.com/buildbarn/bonanza/pkg/proto/storage/object"
	remoteexecution "github.com/buildbarn/bonanza/pkg/remoteexecution"
	"github.com/buildbarn/bonanza/pkg/remoteworker"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"
	object_grpc "github.com/buildbarn/bonanza/pkg/storage/object/grpc"
	object_namespacemapping "github.com/buildbarn/bonanza/pkg/storage/object/namespacemapping"

	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.starlark.net/starlark"
)

func main() {
	program.RunMain(func(ctx context.Context, siblingsGroup, dependenciesGroup program.Group) error {
		if len(os.Args) != 2 {
			return status.Error(codes.InvalidArgument, "Usage: bonanza_builder bonanza_builder.jsonnet")
		}
		var configuration bonanza_builder.ApplicationConfiguration
		if err := util.UnmarshalConfigurationFromFile(os.Args[1], &configuration); err != nil {
			return util.StatusWrapf(err, "Failed to read configuration from %s", os.Args[1])
		}
		lifecycleState, grpcClientFactory, err := global.ApplyConfiguration(configuration.Global)
		if err != nil {
			return util.StatusWrap(err, "Failed to apply global configuration options")
		}

		storageGRPCClient, err := grpcClientFactory.NewClientFromConfiguration(configuration.StorageGrpcClient)
		if err != nil {
			return util.StatusWrap(err, "Failed to create storage gRPC client")
		}
		objectDownloader := object_grpc.NewGRPCDownloader(
			object_pb.NewDownloaderClient(storageGRPCClient),
		)
		parsedObjectPool, err := model_parser.NewParsedObjectPoolFromConfiguration(configuration.ParsedObjectPool)
		if err != nil {
			return util.StatusWrap(err, "Failed to create parsed object pool")
		}

		roundTripper, err := bb_http.NewRoundTripperFromConfiguration(configuration.HttpClient)
		if err != nil {
			return util.StatusWrap(err, "Failed to create HTTP client")
		}

		filePool, err := re_filesystem.NewFilePoolFromConfiguration(configuration.FilePool)
		if err != nil {
			return util.StatusWrap(err, "Failed to create file pool")
		}

		cacheDirectory, err := filesystem.NewLocalDirectory(path.LocalFormat.NewParser(configuration.CacheDirectoryPath))
		if err != nil {
			return util.StatusWrap(err, "Failed to create cache directory")
		}

		executionGRPCClient, err := grpcClientFactory.NewClientFromConfiguration(configuration.ExecutionGrpcClient)
		if err != nil {
			return util.StatusWrap(err, "Failed to create execution gRPC client")
		}

		executionClientPrivateKey, err := remoteexecution.ParseECDHPrivateKey([]byte(configuration.ExecutionClientPrivateKey))
		if err != nil {
			return util.StatusWrap(err, "Failed to parse execution client private key")
		}
		executionClientCertificateChain, err := remoteexecution.ParseCertificateChain([]byte(configuration.ExecutionClientCertificateChain))
		if err != nil {
			return util.StatusWrap(err, "Failed to parse execution client certificate chain")
		}

		remoteWorkerConnection, err := grpcClientFactory.NewClientFromConfiguration(configuration.RemoteWorkerGrpcClient)
		if err != nil {
			return util.StatusWrap(err, "Failed to create remote worker RPC client")
		}
		remoteWorkerClient := remoteworker_pb.NewOperationQueueClient(remoteWorkerConnection)

		platformPrivateKeys, err := remoteworker.ParsePlatformPrivateKeys(configuration.PlatformPrivateKeys)
		if err != nil {
			return err
		}
		clientCertificateAuthorities, err := remoteworker.ParseClientCertificateAuthorities(configuration.ClientCertificateAuthorities)
		if err != nil {
			return err
		}
		workerName, err := json.Marshal(configuration.WorkerId)
		if err != nil {
			return util.StatusWrap(err, "Failed to marshal worker ID")
		}

		bzlFileBuiltins, buildFileBuiltins := model_starlark.GetBuiltins[builderReference, builderReferenceMetadata]()
		executor := &builderExecutor{
			objectDownloader:              objectDownloader,
			parsedObjectPool:              parsedObjectPool,
			dagUploaderClient:             dag_pb.NewUploaderClient(storageGRPCClient),
			objectContentsWalkerSemaphore: semaphore.NewWeighted(int64(runtime.NumCPU())),
			httpClient: &http.Client{
				Transport: bb_http.NewMetricsRoundTripper(roundTripper, "Builder"),
			},
			filePool:       filePool,
			cacheDirectory: cacheDirectory,
			executionClient: remoteexecution.NewClient[*model_command_pb.Action, emptypb.Empty, *model_command_pb.Result](
				remoteexecution_pb.NewExecutionClient(executionGRPCClient),
				executionClientPrivateKey,
				executionClientCertificateChain,
			),
			bzlFileBuiltins:   bzlFileBuiltins,
			buildFileBuiltins: buildFileBuiltins,
		}
		client, err := remoteworker.NewClient(
			remoteWorkerClient,
			executor,
			clock.SystemClock,
			random.CryptoThreadSafeGenerator,
			platformPrivateKeys,
			clientCertificateAuthorities,
			configuration.WorkerId,
			/* sizeClass = */ 0,
			/* isLargestSizeClass = */ true,
		)
		if err != nil {
			return util.StatusWrap(err, "Failed to create remote worker client")
		}
		remoteworker.LaunchWorkerThread(siblingsGroup, client.Run, string(workerName))

		lifecycleState.MarkReadyAndWait(siblingsGroup)
		return nil
	})
}

type referenceWrappingParsedObjectReader struct {
	base model_parser.ParsedObjectReader[object.LocalReference, model_core.Message[[]byte, object.LocalReference]]
}

func (r *referenceWrappingParsedObjectReader) ReadParsedObject(ctx context.Context, reference builderReference) (model_core.Message[[]byte, builderReference], error) {
	if contents := reference.embeddedMetadata.contents; contents != nil {
		// Object has not been written to storage yet.
		// Return the copy that lives in memory.
		//
		// TODO: We should return some kind of hint to indicate
		// that the caller is not permitted to cache this!
		degree := contents.GetDegree()
		outgoingReferences := make(object.OutgoingReferencesList[builderReference], 0, degree)
		children := reference.embeddedMetadata.children
		for i := range degree {
			outgoingReferences = append(outgoingReferences, builderReference{
				LocalReference:   contents.GetOutgoingReference(i),
				embeddedMetadata: children[i],
			})
		}
		return model_core.NewMessage(contents.GetPayload(), outgoingReferences), nil
	}

	// Read object from storage.
	m, err := r.base.ReadParsedObject(ctx, reference.GetLocalReference())
	if err != nil {
		return model_core.Message[[]byte, builderReference]{}, err
	}

	degree := m.OutgoingReferences.GetDegree()
	outgoingReferences := make(object.OutgoingReferencesList[builderReference], 0, degree)
	for i := range degree {
		outgoingReferences = append(outgoingReferences, builderReference{
			LocalReference: m.OutgoingReferences.GetOutgoingReference(i),
		})
	}
	return model_core.NewMessage(m.Message, outgoingReferences), nil
}

func (r *referenceWrappingParsedObjectReader) GetDecodingParametersSizeBytes() int {
	return 0
}

type builderReference struct {
	object.LocalReference
	embeddedMetadata builderReferenceMetadata
}

type builderReferenceMetadata struct {
	contents *object.Contents
	children []builderReferenceMetadata
}

func (builderReferenceMetadata) Discard()     {}
func (builderReferenceMetadata) IsCloneable() {}

func (m builderReferenceMetadata) ToObjectContentsWalker() dag.ObjectContentsWalker {
	return m
}

func (m builderReferenceMetadata) GetContents(ctx context.Context) (*object.Contents, []dag.ObjectContentsWalker, error) {
	if m.contents == nil {
		return nil, nil, status.Error(codes.Internal, "Contents for this object are not available for upload, as this object was expected to already exist")
	}

	walkers := make([]dag.ObjectContentsWalker, 0, len(m.children))
	for _, child := range m.children {
		walkers = append(walkers, child)
	}
	return m.contents, walkers, nil
}

type builderObjectCapturer struct{}

func (builderObjectCapturer) CaptureCreatedObject(createdObject model_core.CreatedObject[builderReferenceMetadata]) builderReferenceMetadata {
	return builderReferenceMetadata{
		contents: createdObject.Contents,
		children: createdObject.Metadata,
	}
}

func (builderObjectCapturer) CaptureExistingObject(reference builderReference) builderReferenceMetadata {
	if reference.embeddedMetadata.contents != nil {
		return reference.embeddedMetadata
	}
	return builderReferenceMetadata{}
}

func (builderObjectCapturer) ReferenceObject(reference object.LocalReference, metadata builderReferenceMetadata) builderReference {
	return builderReference{
		LocalReference:   reference,
		embeddedMetadata: metadata,
	}
}

type builderExecutor struct {
	objectDownloader              object.Downloader[object.GlobalReference]
	parsedObjectPool              *model_parser.ParsedObjectPool
	dagUploaderClient             dag_pb.UploaderClient
	objectContentsWalkerSemaphore *semaphore.Weighted
	httpClient                    *http.Client
	filePool                      re_filesystem.FilePool
	cacheDirectory                filesystem.Directory
	executionClient               *remoteexecution.Client[*model_command_pb.Action, emptypb.Empty, *emptypb.Empty, *model_command_pb.Result]
	bzlFileBuiltins               starlark.StringDict
	buildFileBuiltins             starlark.StringDict
}

func (e *builderExecutor) CheckReadiness(ctx context.Context) error {
	return nil
}

func (e *builderExecutor) Execute(ctx context.Context, action *model_build_pb.Action, executionTimeout time.Duration, executionEvents chan<- proto.Message) (proto.Message, time.Duration, remoteworker_pb.CurrentState_Completed_Result) {
	namespace, err := object.NewNamespace(action.Namespace)
	if err != nil {
		return &model_build_pb.Result{
			Status: status.Convert(util.StatusWrap(err, "Invalid namespace")).Proto(),
		}, 0, remoteworker_pb.CurrentState_Completed_FAILED
	}
	instanceName := namespace.InstanceName
	buildSpecificationReference, err := model_core.NewDecodableLocalReferenceFromWeakProto(namespace.ReferenceFormat, action.BuildSpecificationReference)
	if err != nil {
		return &model_build_pb.Result{
			Status: status.Convert(util.StatusWrap(err, "Invalid build specification reference")).Proto(),
		}, 0, remoteworker_pb.CurrentState_Completed_FAILED
	}
	buildSpecificationEncoder, err := encoding.NewBinaryEncoderFromProto(
		action.BuildSpecificationEncoders,
		uint32(namespace.ReferenceFormat.GetMaximumObjectSizeBytes()),
	)
	if err != nil {
		return &model_build_pb.Result{
			Status: status.Convert(util.StatusWrap(err, "Invalid build specification encoder")).Proto(),
		}, 0, remoteworker_pb.CurrentState_Completed_FAILED
	}

	// Perform the build.
	value, resultsIterator, errCompute := evaluation.FullyComputeValue(
		ctx,
		model_analysis.NewTypedComputer(model_analysis.NewBaseComputer[builderReference, builderReferenceMetadata](
			model_parser.NewParsedObjectPoolIngester[builderReference](
				e.parsedObjectPool,
				&referenceWrappingParsedObjectReader{
					base: model_parser.NewDownloadingParsedObjectReader(
						object_namespacemapping.NewNamespaceAddingDownloader(e.objectDownloader, namespace),
					),
				},
			),
			model_core.CopyDecodable(
				buildSpecificationReference,
				builderReference{
					LocalReference: buildSpecificationReference.Value,
				},
			),
			buildSpecificationEncoder,
			e.httpClient,
			e.filePool,
			e.cacheDirectory,
			e.executionClient,
			namespace.InstanceName,
			e.bzlFileBuiltins,
			e.buildFileBuiltins,
		)),
		model_core.NewSimpleTopLevelMessage[builderReference](proto.Message(&model_analysis_pb.BuildResult_Key{})),
		builderObjectCapturer{},
		func(localReferences object.OutgoingReferences[object.LocalReference], objectContentsWalkers []dag.ObjectContentsWalker) ([]builderReference, error) {
			degree := localReferences.GetDegree()
			storedReferences := make(object.OutgoingReferencesList[builderReference], 0, degree)
			for i := 0; i < degree; i++ {
				localReference := localReferences.GetOutgoingReference(i)
				if err := dag.UploadDAG(
					ctx,
					e.dagUploaderClient,
					object.GlobalReference{
						InstanceName:   instanceName,
						LocalReference: localReference,
					},
					objectContentsWalkers[i],
					e.objectContentsWalkerSemaphore,
					// Assume everything we attempt
					// to upload is memory backed.
					object.Unlimited,
				); err != nil {
					return nil, fmt.Errorf("failed to store DAG with reference %s: %w", localReference.String(), err)
				}
				storedReferences = append(storedReferences, builderReference{
					LocalReference: localReference,
				})
			}
			return storedReferences, nil
		},
	)

	// Store all evaluation results to permit debugging of the build.
	// TODO: Use a proper configuration.
	evaluationTreeEncoder := model_encoding.NewChainedBinaryEncoder(nil)
	evaluationTreeBuilder := btree.NewSplitProllyBuilder(
		1<<16,
		1<<18,
		btree.NewObjectCreatingNodeMerger(
			evaluationTreeEncoder,
			namespace.ReferenceFormat,
			/* parentNodeComputer = */ func(createdObject model_core.Decodable[model_core.CreatedObject[dag.ObjectContentsWalker]], childNodes []*model_evaluation_pb.Evaluation) (model_core.PatchedMessage[*model_evaluation_pb.Evaluation, dag.ObjectContentsWalker], error) {
				var firstKey []byte
				switch firstEntry := childNodes[0].Level.(type) {
				case *model_evaluation_pb.Evaluation_Leaf_:
					flattenedAny, err := model_core.FlattenAny(model_core.NewMessage(firstEntry.Leaf.Key, createdObject.Value.Contents))
					if err != nil {
						return model_core.PatchedMessage[*model_evaluation_pb.Evaluation, dag.ObjectContentsWalker]{}, err
					}
					firstKey, err = model_core.MarshalTopLevelMessage(flattenedAny)
					if err != nil {
						return model_core.PatchedMessage[*model_evaluation_pb.Evaluation, dag.ObjectContentsWalker]{}, err
					}
				case *model_evaluation_pb.Evaluation_Parent_:
					firstKey = firstEntry.Parent.FirstKey
				}
				patcher := model_core.NewReferenceMessagePatcher[dag.ObjectContentsWalker]()
				return model_core.NewPatchedMessage(
					&model_evaluation_pb.Evaluation{
						Level: &model_evaluation_pb.Evaluation_Parent_{
							Parent: &model_evaluation_pb.Evaluation_Parent{
								Reference: patcher.CaptureAndAddDecodableReference(createdObject, model_core.WalkableCreatedObjectCapturer),
								FirstKey:  firstKey,
							},
						},
					},
					patcher,
				), nil
			},
		),
	)
	for evaluation := range resultsIterator {
		key, err := model_core.MarshalAny(
			model_core.NewPatchedMessageFromExisting(
				evaluation.Key,
				func(index int) dag.ObjectContentsWalker {
					return dag.ExistingObjectContentsWalker
				},
			),
		)
		if err != nil {
			return &model_build_pb.Result{
				Status: status.Convert(err).Proto(),
			}, 0, remoteworker_pb.CurrentState_Completed_FAILED
		}
		patcher := key.Patcher

		var value model_core.PatchedMessage[*model_core_pb.Any, dag.ObjectContentsWalker]
		if evaluation.Value.IsSet() {
			value, err = model_core.MarshalAny(
				model_core.NewPatchedMessageFromExisting(
					evaluation.Value,
					func(index int) dag.ObjectContentsWalker {
						return dag.ExistingObjectContentsWalker
					},
				),
			)
			if err != nil {
				return &model_build_pb.Result{
					Status: status.Convert(err).Proto(),
				}, 0, remoteworker_pb.CurrentState_Completed_FAILED
			}
			patcher.Merge(value.Patcher)
		}

		dependencyTreeBuilder := btree.NewSplitProllyBuilder(
			1<<16,
			1<<18,
			btree.NewObjectCreatingNodeMerger(
				evaluationTreeEncoder,
				namespace.ReferenceFormat,
				/* parentNodeComputer = */ func(createdObject model_core.Decodable[model_core.CreatedObject[dag.ObjectContentsWalker]], childNodes []*model_evaluation_pb.Dependency) (model_core.PatchedMessage[*model_evaluation_pb.Dependency, dag.ObjectContentsWalker], error) {
					patcher := model_core.NewReferenceMessagePatcher[dag.ObjectContentsWalker]()
					return model_core.NewPatchedMessage(
						&model_evaluation_pb.Dependency{
							Level: &model_evaluation_pb.Dependency_Parent_{
								Parent: &model_evaluation_pb.Dependency_Parent{
									Reference: patcher.CaptureAndAddDecodableReference(createdObject, model_core.WalkableCreatedObjectCapturer),
								},
							},
						},
						patcher,
					), nil
				},
			),
		)
		for _, dependency := range evaluation.Dependencies {
			dependencyAny, err := model_core.MarshalAny(
				model_core.NewPatchedMessageFromExisting(
					dependency,
					func(index int) dag.ObjectContentsWalker {
						return dag.ExistingObjectContentsWalker
					},
				),
			)
			if err != nil {
				return &model_build_pb.Result{
					Status: status.Convert(err).Proto(),
				}, 0, remoteworker_pb.CurrentState_Completed_FAILED
			}
			if err := dependencyTreeBuilder.PushChild(
				model_core.NewPatchedMessage(
					&model_evaluation_pb.Dependency{
						Level: &model_evaluation_pb.Dependency_LeafKey{
							LeafKey: dependencyAny.Message,
						},
					},
					dependencyAny.Patcher,
				),
			); err != nil {
				return &model_build_pb.Result{
					Status: status.Convert(err).Proto(),
				}, 0, remoteworker_pb.CurrentState_Completed_FAILED
			}
		}
		dependencies, err := dependencyTreeBuilder.FinalizeList()
		if err != nil {
			return &model_build_pb.Result{
				Status: status.Convert(err).Proto(),
			}, 0, remoteworker_pb.CurrentState_Completed_FAILED
		}
		patcher.Merge(dependencies.Patcher)

		if err := evaluationTreeBuilder.PushChild(
			model_core.NewPatchedMessage(
				&model_evaluation_pb.Evaluation{
					Level: &model_evaluation_pb.Evaluation_Leaf_{
						Leaf: &model_evaluation_pb.Evaluation_Leaf{
							Key:          key.Message,
							Value:        value.Message,
							Dependencies: dependencies.Message,
						},
					},
				},
				patcher,
			),
		); err != nil {
			return &model_build_pb.Result{
				Status: status.Convert(err).Proto(),
			}, 0, remoteworker_pb.CurrentState_Completed_FAILED
		}
	}
	evaluations, err := evaluationTreeBuilder.FinalizeList()
	if err != nil {
		return &model_build_pb.Result{
			Status: status.Convert(err).Proto(),
		}, 0, remoteworker_pb.CurrentState_Completed_FAILED
	}
	if len(evaluations.Message) > 0 {
		createdEvaluations, err := model_core.MarshalAndEncodePatchedListMessage(
			evaluations,
			namespace.ReferenceFormat,
			evaluationTreeEncoder,
		)
		if err != nil {
			return &model_build_pb.Result{
				Status: status.Convert(err).Proto(),
			}, 0, remoteworker_pb.CurrentState_Completed_FAILED
		}
		createdEvaluationsReference := createdEvaluations.Value.Contents.GetReference()
		if err := dag.UploadDAG(
			ctx,
			e.dagUploaderClient,
			object.GlobalReference{
				InstanceName:   instanceName,
				LocalReference: createdEvaluationsReference,
			},
			model_core.WalkableCreatedObjectCapturer.CaptureCreatedObject(createdEvaluations.Value),
			e.objectContentsWalkerSemaphore,
			// Assume everything we attempt
			// to upload is memory backed.
			object.Unlimited,
		); err != nil {
			return &model_build_pb.Result{
				Status: status.Convert(errCompute).Proto(),
			}, 0, remoteworker_pb.CurrentState_Completed_FAILED
		}

		// TODO: This reference should be embedded in the
		// response that is sent back to the client.
		log.Print(
			"Evaluations: ",
			model_core.DecodableLocalReferenceToString(
				model_core.CopyDecodable(
					createdEvaluations,
					createdEvaluationsReference,
				),
			),
		)
	}

	if errCompute != nil {
		return &model_build_pb.Result{
			Status: status.Convert(errCompute).Proto(),
		}, 0, remoteworker_pb.CurrentState_Completed_FAILED
	}
	return &model_build_pb.Result{
		Status: status.Newf(codes.Internal, "TODO: %s", value).Proto(),
	}, 0, remoteworker_pb.CurrentState_Completed_FAILED
}

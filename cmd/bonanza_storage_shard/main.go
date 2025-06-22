package main

import (
	"context"
	"math/rand/v2"
	"os"

	"bonanza.build/pkg/ds/lossymap"
	"bonanza.build/pkg/proto/configuration/bonanza_storage_shard"
	object_pb "bonanza.build/pkg/proto/storage/object"
	tag_pb "bonanza.build/pkg/proto/storage/tag"
	"bonanza.build/pkg/storage/object"
	object_flatbacked "bonanza.build/pkg/storage/object/flatbacked"
	object_leasemarshaling "bonanza.build/pkg/storage/object/leasemarshaling"
	object_local "bonanza.build/pkg/storage/object/local"
	object_namespacemapping "bonanza.build/pkg/storage/object/namespacemapping"
	"bonanza.build/pkg/storage/tag"
	tag_leasemarshaling "bonanza.build/pkg/storage/tag/leasemarshaling"
	tag_local "bonanza.build/pkg/storage/tag/local"

	"github.com/buildbarn/bb-storage/pkg/clock"
	"github.com/buildbarn/bb-storage/pkg/global"
	bb_grpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/program"
	"github.com/buildbarn/bb-storage/pkg/util"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	program.RunMain(func(ctx context.Context, siblingsGroup, dependenciesGroup program.Group) error {
		if len(os.Args) != 2 {
			return status.Error(codes.InvalidArgument, "Usage: bonanza_storage_shard bonanza_storage_shard.jsonnet")
		}
		var configuration bonanza_storage_shard.ApplicationConfiguration
		if err := util.UnmarshalConfigurationFromFile(os.Args[1], &configuration); err != nil {
			return util.StatusWrapf(err, "Failed to read configuration from %s", os.Args[1])
		}
		lifecycleState, grpcClientFactory, err := global.ApplyConfiguration(configuration.Global)
		if err != nil {
			return util.StatusWrap(err, "Failed to apply global configuration options")
		}

		// Construct a flat storage backend for objects. Put an
		// in-memory store for leases in front of it, so that
		// UploadDag() can reliably enforce that clients upload
		// complete DAGs.
		localObjectStore, err := object_local.NewStoreFromConfiguration(
			dependenciesGroup,
			configuration.LocalObjectStore,
		)
		if err != nil {
			return util.StatusWrap(err, "Failed to create local object store")
		}

		leaseCompletenessDuration := configuration.LeasesMapLeaseCompletenessDuration
		if err := leaseCompletenessDuration.CheckValid(); err != nil {
			return util.StatusWrap(err, "Invalid leases map lease completeness duration")
		}
		leaseComparator := func(a, b *object_flatbacked.Lease) int {
			if *a < *b {
				return -1
			}
			if *a > *b {
				return 1
			}
			return 0
		}
		flatBackedObjectStoreHashInitialization := rand.Uint64()
		flatBackedObjectStore := object_flatbacked.NewStore(
			localObjectStore,
			clock.SystemClock,
			leaseCompletenessDuration.AsDuration(),
			lossymap.NewHashMap(
				lossymap.NewSimpleRecordArray[object.LocalReference](
					int(configuration.LeasesMapRecordsCount),
					leaseComparator,
				),
				/* recordKeyHasher = */ func(k *lossymap.RecordKey[object.LocalReference]) uint64 {
					h := flatBackedObjectStoreHashInitialization
					for _, c := range k.Key.GetRawReference() {
						h ^= uint64(c)
						h *= 1099511628211
					}
					attempt := k.Attempt
					for i := 0; i < 4; i++ {
						h ^= uint64(attempt & 0xff)
						h *= 1099511628211
						attempt >>= 8
					}
					return h
				},
				configuration.LeasesMapRecordsCount,
				leaseComparator,
				uint8(configuration.LeasesMapMaximumGetAttempts),
				int(configuration.LeasesMapMaximumPutAttempts),
				"LeasesMap",
			),
		)
		tagStore := tag_local.NewStore()
		leaseMarshaler := object_flatbacked.LeaseMarshaler

		if err := bb_grpc.NewServersFromConfigurationAndServe(
			configuration.GrpcServers,
			func(s grpc.ServiceRegistrar) {
				object_pb.RegisterDownloaderServer(
					s,
					object.NewDownloaderServer(
						object_namespacemapping.NewNamespaceRemovingDownloader[object.GlobalReference](
							flatBackedObjectStore,
						),
					),
				)
				object_pb.RegisterUploaderServer(
					s,
					object.NewUploaderServer(
						object_leasemarshaling.NewUploader(
							object_namespacemapping.NewNamespaceRemovingUploader[object.GlobalReference](
								flatBackedObjectStore,
							),
							leaseMarshaler,
						),
					),
				)
				tag_pb.RegisterResolverServer(
					s,
					tag.NewResolverServer(
						tagStore,
					),
				)
				tag_pb.RegisterUpdaterServer(
					s,
					tag.NewUpdaterServer(
						tag_leasemarshaling.NewUpdater(
							tagStore,
							leaseMarshaler,
						),
					),
				)
			},
			siblingsGroup,
			grpcClientFactory,
		); err != nil {
			return util.StatusWrap(err, "gRPC server failure")
		}

		lifecycleState.MarkReadyAndWait(siblingsGroup)
		return nil
	})
}

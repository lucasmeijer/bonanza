package main

import (
	"context"
	"net/http"
	"os"

	"github.com/buildbarn/bb-storage/pkg/global"
	bb_http "github.com/buildbarn/bb-storage/pkg/http"
	"github.com/buildbarn/bb-storage/pkg/program"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/buildbarn/bonanza/pkg/proto/configuration/bonanza_browser"
	object_pb "github.com/buildbarn/bonanza/pkg/proto/storage/object"
	object_grpc "github.com/buildbarn/bonanza/pkg/storage/object/grpc"

	// Included to ensure message of these types are displayable.
	_ "github.com/buildbarn/bonanza/pkg/proto/model/analysis"
	_ "github.com/buildbarn/bonanza/pkg/proto/model/command"
	_ "github.com/buildbarn/bonanza/pkg/proto/model/core"
	_ "github.com/buildbarn/bonanza/pkg/proto/model/evaluation"
	_ "github.com/buildbarn/bonanza/pkg/proto/model/filesystem"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	program.RunMain(func(ctx context.Context, siblingsGroup, dependenciesGroup program.Group) error {
		if len(os.Args) != 2 {
			return status.Error(codes.InvalidArgument, "Usage: bonanza_browser bonanza_browser.jsonnet")
		}
		var configuration bonanza_browser.ApplicationConfiguration
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

		mux := http.NewServeMux()
		browserService := NewBrowserService(objectDownloader)
		browserService.RegisterHandlers(mux)
		bb_http.NewServersFromConfigurationAndServe(
			configuration.HttpServers,
			bb_http.NewMetricsHandler(mux, "BrowserUI"),
			siblingsGroup,
			grpcClientFactory,
		)

		lifecycleState.MarkReadyAndWait(siblingsGroup)
		return nil
	})
}

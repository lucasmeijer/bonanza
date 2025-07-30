package existenceprecondition

import (
	"context"

	"bonanza.build/pkg/storage/object"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type downloader[TReference any] struct {
	base object.Downloader[TReference]
}

// NewDownloader creates a decorator for object.Downloader that causes
// all NOT_FOUND errors returned by DownloadObject() to be translated to
// FAILED_PRECONDITION.
//
// This decorator can be used on workers that expect that all objects
// references by actions that they receive are present in storage. If
// objects cannot be read from storage, it's not desirable to let the
// execution of the action itself fail with NOT_FOUND.
func NewDownloader[TReference any](base object.Downloader[TReference]) object.Downloader[TReference] {
	return &downloader[TReference]{
		base: base,
	}
}

func (d *downloader[TReference]) DownloadObject(ctx context.Context, reference TReference) (*object.Contents, error) {
	contents, err := d.base.DownloadObject(ctx, reference)
	if s := status.Convert(err); s.Code() == codes.NotFound {
		return nil, status.Error(codes.FailedPrecondition, s.Message())
	}
	return contents, err
}

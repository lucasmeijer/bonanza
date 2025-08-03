package suspending

import (
	"context"

	"bonanza.build/pkg/storage/object"

	"github.com/buildbarn/bb-remote-execution/pkg/clock"
)

type downloader[TReference any] struct {
	base        object.Downloader[TReference]
	suspendable clock.Suspendable
}

// NewDownloader creates a decorator for object.Downloader that suspends
// the progression of timers while downloads of objects are taking
// place. This may be used by workers to ensure that any delays
// downloading objects from storage does not lead to actions timing out
// prematurely.
func NewDownloader[TReference any](base object.Downloader[TReference], suspendable clock.Suspendable) object.Downloader[TReference] {
	return &downloader[TReference]{
		base:        base,
		suspendable: suspendable,
	}
}

func (d *downloader[TReference]) DownloadObject(ctx context.Context, reference TReference) (*object.Contents, error) {
	d.suspendable.Suspend()
	defer d.suspendable.Resume()
	return d.base.DownloadObject(ctx, reference)
}

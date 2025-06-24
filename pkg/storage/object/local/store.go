package local

import (
	"context"
	"sync"

	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type store struct {
	lock    sync.Mutex
	objects map[object.FlatReference]*object.Contents
}

// NewStore creates an object store that uses locally connected disks as
// its backing store.
func NewStore() object.Store[object.FlatReference, struct{}] {
	return &store{
		objects: map[object.FlatReference]*object.Contents{},
	}
}

func (s *store) DownloadObject(ctx context.Context, reference object.FlatReference) (*object.Contents, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	contents, ok := s.objects[reference]
	if !ok {
		return nil, status.Error(codes.NotFound, "Object not found")
	}
	return contents, nil
}

func (s *store) UploadObject(ctx context.Context, reference object.FlatReference, contents *object.Contents, childrenLeases []struct{}, wantContentsIfIncomplete bool) (object.UploadObjectResult[struct{}], error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.objects[reference]; !ok {
		if contents == nil {
			return object.UploadObjectMissing[struct{}]{}, nil
		}
		s.objects[reference] = contents
	}
	return object.UploadObjectComplete[struct{}]{}, nil
}

package command

import (
	"iter"
	"maps"
	"slices"

	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/model/core/inlinedtree"
	model_command_pb "github.com/buildbarn/bonanza/pkg/proto/model/command"

	"google.golang.org/protobuf/proto"
)

func GetPathPatternWithChildren[TMetadata model_core.ReferenceMetadata](
	children model_core.PatchedMessage[*model_command_pb.PathPattern_Children, TMetadata],
	externalObject *model_core.Decodable[model_core.CreatedObject[TMetadata]],
	patcher *model_core.ReferenceMessagePatcher[TMetadata],
	objectCapturer model_core.CreatedObjectCapturer[TMetadata],
) *model_command_pb.PathPattern {
	if children.Message == nil {
		return &model_command_pb.PathPattern{}
	}
	if externalObject == nil {
		return &model_command_pb.PathPattern{
			Children: &model_command_pb.PathPattern_ChildrenInline{
				ChildrenInline: children.Message,
			},
		}
	}
	return &model_command_pb.PathPattern{
		Children: &model_command_pb.PathPattern_ChildrenExternal{
			ChildrenExternal: patcher.CaptureAndAddDecodableReference(
				*externalObject,
				objectCapturer,
			),
		},
	}
}

func GetPathPatternInlineCandidate[TMetadata model_core.ReferenceMetadata](name string, grandChildren model_core.PatchedMessage[*model_command_pb.PathPattern_Children, TMetadata], objectCapturer model_core.CreatedObjectCapturer[TMetadata]) inlinedtree.Candidate[*model_command_pb.PathPattern_Children, TMetadata] {
	return inlinedtree.Candidate[*model_command_pb.PathPattern_Children, TMetadata]{
		ExternalMessage: model_core.NewPatchedMessage[proto.Message](grandChildren.Message, grandChildren.Patcher),
		ParentAppender: func(
			children model_core.PatchedMessage[*model_command_pb.PathPattern_Children, TMetadata],
			externalObject *model_core.Decodable[model_core.CreatedObject[TMetadata]],
		) {
			children.Message.Children = append(children.Message.Children, &model_command_pb.PathPattern_Child{
				Name:    name,
				Pattern: GetPathPatternWithChildren(grandChildren, externalObject, children.Patcher, objectCapturer),
			})
		},
	}
}

func PrependDirectoryToPathPatternChildren[TMetadata model_core.ReferenceMetadata](name string, grandChildren model_core.PatchedMessage[*model_command_pb.PathPattern_Children, TMetadata], inlinedTreeOptions *inlinedtree.Options, objectCapturer model_core.CreatedObjectCapturer[TMetadata]) (model_core.PatchedMessage[*model_command_pb.PathPattern_Children, TMetadata], error) {
	return inlinedtree.Build(
		inlinedtree.CandidateList[*model_command_pb.PathPattern_Children, TMetadata]{
			GetPathPatternInlineCandidate(name, grandChildren, objectCapturer),
		},
		inlinedTreeOptions,
	)
}

// PathPatternSet is a set of relative pathname strings that should be
// captured by a remote worker after execution of an action completes.
type PathPatternSet[TMetadata model_core.ReferenceMetadata] struct {
	children map[string]*PathPatternSet[TMetadata]
	included bool
}

func (s *PathPatternSet[TMetadata]) Add(path iter.Seq[string]) {
	for component := range path {
		sChild, ok := s.children[component]
		if !ok {
			if s.children == nil {
				s.children = map[string]*PathPatternSet[TMetadata]{}
			}
			sChild = &PathPatternSet[TMetadata]{}
			s.children[component] = sChild
		}
		s = sChild
	}
	s.included = true
}

// ToProto converts the set of relative pathname strings contained in
// the set to a PathPattern message that can be embedded in a Command
// message, which is to be processed by a remote worker.
func (s *PathPatternSet[TMetadata]) ToProto(inlinedTreeOptions *inlinedtree.Options, objectCapturer model_core.CreatedObjectCapturer[TMetadata]) (model_core.PatchedMessage[*model_command_pb.PathPattern_Children, TMetadata], error) {
	if s.included {
		return model_core.NewSimplePatchedMessage[TMetadata]((*model_command_pb.PathPattern_Children)(nil)), nil
	}

	if len(s.children) == 0 {
		panic("leaf path should have been included in the path pattern set")
	}
	inlineCandidates := make(inlinedtree.CandidateList[*model_command_pb.PathPattern_Children, TMetadata], 0, len(s.children))
	for _, name := range slices.Sorted(maps.Keys(s.children)) {
		grandChildren, err := s.children[name].ToProto(inlinedTreeOptions, objectCapturer)
		if err != nil {
			return model_core.PatchedMessage[*model_command_pb.PathPattern_Children, TMetadata]{}, err
		}
		inlineCandidates = append(inlineCandidates, GetPathPatternInlineCandidate(name, grandChildren, objectCapturer))
	}
	return inlinedtree.Build(inlineCandidates, inlinedTreeOptions)
}

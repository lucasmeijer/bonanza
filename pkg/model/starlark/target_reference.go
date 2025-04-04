package starlark

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	pg_label "github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

type TargetReference[TReference object.BasicReference, TMetadata model_core.CloneableReferenceMetadata] struct {
	label            pg_label.ResolvedLabel
	encodedProviders model_core.Message[[]*model_starlark_pb.Struct, TReference]

	decodedProviders []atomic.Pointer[Struct[TReference, TMetadata]]
}

func NewTargetReference[TReference object.BasicReference, TMetadata model_core.CloneableReferenceMetadata](label pg_label.ResolvedLabel, providers model_core.Message[[]*model_starlark_pb.Struct, TReference]) starlark.Value {
	return &TargetReference[TReference, TMetadata]{
		label:            label,
		encodedProviders: providers,
		decodedProviders: make([]atomic.Pointer[Struct[TReference, TMetadata]], len(providers.Message)),
	}
}

var (
	_ EncodableValue[object.LocalReference, model_core.CloneableReferenceMetadata] = (*TargetReference[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
	_ starlark.Comparable                                                          = (*TargetReference[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
	_ starlark.HasAttrs                                                            = (*TargetReference[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
	_ starlark.Mapping                                                             = (*TargetReference[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
)

func (tr *TargetReference[TReference, TMetadata]) String() string {
	return fmt.Sprintf("<target %s>", tr.label.String())
}

func (TargetReference[TReference, TMetadata]) Type() string {
	return "Target"
}

func (TargetReference[TReference, TMetadata]) Freeze() {
}

func (TargetReference[TReference, TMetadata]) Truth() starlark.Bool {
	return starlark.True
}

func (tr *TargetReference[TReference, TMetadata]) Hash(thread *starlark.Thread) (uint32, error) {
	// Assume that the number of target references with the same
	// label, but a different configuration are fairly low.
	return starlark.String(tr.label.String()).Hash(thread)
}

func (tr *TargetReference[TReference, TMetadata]) equal(thread *starlark.Thread, other *TargetReference[TReference, TMetadata]) (bool, error) {
	if tr != other {
		if tr.label != other.label {
			return false, nil
		}
		if len(tr.encodedProviders.Message) != len(other.encodedProviders.Message) {
			return false, nil
		}
		return false, errors.New("TODO: Compare encoded providers!")
	}
	return true, nil
}

func (tr *TargetReference[TReference, TMetadata]) CompareSameType(thread *starlark.Thread, op syntax.Token, other starlark.Value, depth int) (bool, error) {
	switch op {
	case syntax.EQL:
		return tr.equal(thread, other.(*TargetReference[TReference, TMetadata]))
	case syntax.NEQ:
		equals, err := tr.equal(thread, other.(*TargetReference[TReference, TMetadata]))
		return !equals, err
	default:
		return false, errors.New("target references cannot be compared for inequality")
	}
}

var defaultInfoProviderIdentifier = pg_label.MustNewCanonicalStarlarkIdentifier("@@builtins_core+//:exports.bzl%DefaultInfo")

func (tr *TargetReference[TReference, TMetadata]) Attr(thread *starlark.Thread, name string) (starlark.Value, error) {
	switch name {
	case "label":
		return NewLabel[TReference, TMetadata](tr.label), nil
	case "data_runfiles", "default_runfiles", "files", "files_to_run":
		// Fields provided by DefaultInfo can be accessed directly.
		defaultInfoProviderValue, err := tr.getProviderValue(thread, defaultInfoProviderIdentifier)
		if err != nil {
			return nil, err
		}
		return defaultInfoProviderValue.Attr(thread, name)
	default:
		return nil, nil
	}
}

var targetReferenceAttrNames = []string{
	"data_runfiles",
	"default_runfiles",
	"files",
	"files_to_run",
	"label",
}

func (tr *TargetReference[TReference, TMetadata]) AttrNames() []string {
	return targetReferenceAttrNames
}

func (tr *TargetReference[TReference, TMetadata]) getProviderValue(thread *starlark.Thread, providerIdentifier pg_label.CanonicalStarlarkIdentifier) (*Struct[TReference, TMetadata], error) {
	valueDecodingOptions := thread.Local(ValueDecodingOptionsKey)
	if valueDecodingOptions == nil {
		return nil, errors.New("providers cannot be decoded from within this context")
	}

	providerIdentifierStr := providerIdentifier.String()
	index, ok := sort.Find(
		len(tr.encodedProviders.Message),
		func(i int) int {
			return strings.Compare(providerIdentifierStr, tr.encodedProviders.Message[i].ProviderInstanceProperties.GetProviderIdentifier())
		},
	)
	if !ok {
		return nil, fmt.Errorf("target %#v did not yield provider %#v", tr.label.String(), providerIdentifierStr)
	}

	strukt := tr.decodedProviders[index].Load()
	if strukt == nil {
		var err error
		strukt, err = DecodeStruct[TReference, TMetadata](
			model_core.Nested(tr.encodedProviders, tr.encodedProviders.Message[index]),
			valueDecodingOptions.(*ValueDecodingOptions[TReference]),
		)
		if err != nil {
			return nil, err
		}
		tr.decodedProviders[index].Store(strukt)
	}
	return strukt, nil
}

func (tr *TargetReference[TReference, TMetadata]) Get(thread *starlark.Thread, v starlark.Value) (starlark.Value, bool, error) {
	provider, ok := v.(*Provider[TReference, TMetadata])
	if !ok {
		return nil, false, errors.New("keys have to be of type provider")
	}
	providerIdentifier := provider.Identifier
	if providerIdentifier == nil {
		return nil, false, errors.New("provider does not have a name")
	}
	providerValue, err := tr.getProviderValue(thread, *providerIdentifier)
	if err != nil {
		return nil, false, err
	}
	return providerValue, true, nil
}

func (tr *TargetReference[TReference, TMetadata]) EncodeValue(path map[starlark.Value]struct{}, currentIdentifier *pg_label.CanonicalStarlarkIdentifier, options *ValueEncodingOptions[TReference, TMetadata]) (model_core.PatchedMessage[*model_starlark_pb.Value, TMetadata], bool, error) {
	return model_core.Patch(
		options.ObjectCapturer,
		model_core.Nested(tr.encodedProviders, &model_starlark_pb.Value{
			Kind: &model_starlark_pb.Value_TargetReference{
				TargetReference: &model_starlark_pb.TargetReference{
					Label:     tr.label.String(),
					Providers: tr.encodedProviders.Message,
				},
			},
		}),
	), false, nil
}

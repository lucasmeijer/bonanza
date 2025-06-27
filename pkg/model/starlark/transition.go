package starlark

import (
	"errors"
	"fmt"

	pg_label "bonanza.build/pkg/label"
	model_core "bonanza.build/pkg/model/core"
	model_starlark_pb "bonanza.build/pkg/proto/model/starlark"
	"bonanza.build/pkg/starlark/unpack"
	"bonanza.build/pkg/storage/object"

	"google.golang.org/protobuf/types/known/emptypb"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// Transition is a Starlark value type that corresponds to a predeclared
// or user defined transition. Transitions can be used to mutate a
// configuration, either as part of an incoming or outgoing edge in the
// build graph.
type Transition[TReference any, TMetadata model_core.CloneableReferenceMetadata] struct {
	TransitionDefinition[TReference, TMetadata]
}

var (
	_ EncodableValue[object.LocalReference, model_core.CloneableReferenceMetadata] = (*Transition[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
	_ NamedGlobal                                                                  = (*Transition[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
)

// NewTransition creates a new Starlark transition value having a given
// definition. This function is typically invoked when exec_transition()
// or transition() is called.
func NewTransition[TReference any, TMetadata model_core.CloneableReferenceMetadata](definition TransitionDefinition[TReference, TMetadata]) starlark.Value {
	return &Transition[TReference, TMetadata]{
		TransitionDefinition: definition,
	}
}

func (Transition[TReference, TMetadata]) String() string {
	return "<transition>"
}

// Type returns a string representation of the type of a transition.
func (Transition[TReference, TMetadata]) Type() string {
	return "transition"
}

// Freeze the definition of the transition. As transitions are
// immutable, this function has no effect.
func (Transition[TReference, TMetadata]) Freeze() {}

// Truth returns whether the transition is a truthy or a falsy.
// Transitions are always truthy.
func (Transition[TReference, TMetadata]) Truth() starlark.Bool {
	return starlark.True
}

// Hash the transition object, so that it may be used as the key in a
// dictionary. This is currently not supported.
func (Transition[TReference, TMetadata]) Hash(thread *starlark.Thread) (uint32, error) {
	return 0, errors.New("transition cannot be hashed")
}

// TransitionDefinition contains the definition of a configuration
// transition. For user defined transitions this may contain all of the
// transition's properties (inputs, outputs, reference to an
// implementation function). For predeclared transitions ("exec",
// "target", config.none(), etc.), the definition may be trivial.
type TransitionDefinition[TReference any, TMetadata model_core.CloneableReferenceMetadata] interface {
	EncodableValue[TReference, TMetadata]
	AssignIdentifier(identifier pg_label.CanonicalStarlarkIdentifier)
	EncodeReference() (*model_starlark_pb.Transition_Reference, error)
	GetUserDefinedTransitionIdentifier() (string, error)
}

type referenceTransitionDefinition[TReference any, TMetadata model_core.CloneableReferenceMetadata] struct {
	reference *model_starlark_pb.Transition_Reference
}

// NewReferenceTransitionDefinition creates a reference to a transition.
// These may either refer to a user defined transition using its
// Starlark identifier, or a predeclared transition ("exec", "target",
// config.none(), etc.).
func NewReferenceTransitionDefinition[TReference any, TMetadata model_core.CloneableReferenceMetadata](reference *model_starlark_pb.Transition_Reference) TransitionDefinition[TReference, TMetadata] {
	return &referenceTransitionDefinition[TReference, TMetadata]{
		reference: reference,
	}
}

func (referenceTransitionDefinition[TReference, TMetadata]) AssignIdentifier(identifier pg_label.CanonicalStarlarkIdentifier) {
}

func (td *referenceTransitionDefinition[TReference, TMetadata]) EncodeReference() (*model_starlark_pb.Transition_Reference, error) {
	return td.reference, nil
}

func (td *referenceTransitionDefinition[TReference, TMetadata]) GetUserDefinedTransitionIdentifier() (string, error) {
	userDefined, ok := td.reference.Kind.(*model_starlark_pb.Transition_Reference_UserDefined)
	if !ok {
		return "", errors.New("transition is not a user-defined transition")
	}
	return userDefined.UserDefined, nil
}

func (td *referenceTransitionDefinition[TReference, TMetadata]) EncodeValue(path map[starlark.Value]struct{}, currentIdentifier *pg_label.CanonicalStarlarkIdentifier, options *ValueEncodingOptions[TReference, TMetadata]) (model_core.PatchedMessage[*model_starlark_pb.Value, TMetadata], bool, error) {
	return model_core.NewSimplePatchedMessage[TMetadata](
		&model_starlark_pb.Value{
			Kind: &model_starlark_pb.Value_Transition{
				Transition: &model_starlark_pb.Transition{
					Kind: &model_starlark_pb.Transition_Reference_{
						Reference: td.reference,
					},
				},
			},
		},
	), false, nil
}

// References to transitions that are used frequently (e.g., "exec",
// "target", config.none()).
var (
	DefaultExecGroupTransitionReference = model_starlark_pb.Transition_Reference{
		Kind: &model_starlark_pb.Transition_Reference_ExecGroup{
			ExecGroup: "",
		},
	}
	NoneTransitionReference = model_starlark_pb.Transition_Reference{
		Kind: &model_starlark_pb.Transition_Reference_None{
			None: &emptypb.Empty{},
		},
	}
	TargetTransitionReference = model_starlark_pb.Transition_Reference{
		Kind: &model_starlark_pb.Transition_Reference_Target{
			Target: &emptypb.Empty{},
		},
	}
	UnconfiguredTransitionReference = model_starlark_pb.Transition_Reference{
		Kind: &model_starlark_pb.Transition_Reference_Unconfigured{
			Unconfigured: &emptypb.Empty{},
		},
	}
)

type userDefinedTransitionDefinition[TReference any, TMetadata model_core.CloneableReferenceMetadata] struct {
	LateNamedValue

	implementation NamedFunction[TReference, TMetadata]
	inputs         []string
	outputs        []string
}

// NewUserDefinedTransitionDefinition creates an object holding the
// properties of a new user defined transition, as normally done by
// exec_transition() or transition().
func NewUserDefinedTransitionDefinition[TReference any, TMetadata model_core.CloneableReferenceMetadata](identifier *pg_label.CanonicalStarlarkIdentifier, implementation NamedFunction[TReference, TMetadata], inputs, outputs []string) TransitionDefinition[TReference, TMetadata] {
	return &userDefinedTransitionDefinition[TReference, TMetadata]{
		LateNamedValue: LateNamedValue{
			Identifier: identifier,
		},
		implementation: implementation,
		inputs:         inputs,
		outputs:        outputs,
	}
}

func (td *userDefinedTransitionDefinition[TReference, TMetadata]) EncodeReference() (*model_starlark_pb.Transition_Reference, error) {
	if td.Identifier == nil {
		return nil, errors.New("transition does not have a name")
	}
	return &model_starlark_pb.Transition_Reference{
		Kind: &model_starlark_pb.Transition_Reference_UserDefined{
			UserDefined: td.Identifier.String(),
		},
	}, nil
}

func (td *userDefinedTransitionDefinition[TReference, TMetadata]) GetUserDefinedTransitionIdentifier() (string, error) {
	if td.Identifier == nil {
		return "", errors.New("transition does not have a name")
	}
	return td.Identifier.String(), nil
}

func (td *userDefinedTransitionDefinition[TReference, TMetadata]) EncodeValue(path map[starlark.Value]struct{}, currentIdentifier *pg_label.CanonicalStarlarkIdentifier, options *ValueEncodingOptions[TReference, TMetadata]) (model_core.PatchedMessage[*model_starlark_pb.Value, TMetadata], bool, error) {
	if td.Identifier == nil {
		return model_core.PatchedMessage[*model_starlark_pb.Value, TMetadata]{}, false, errors.New("transition does not have a name")
	}
	if currentIdentifier == nil || *currentIdentifier != *td.Identifier {
		// Not the canonical identifier under which this
		// transition is known. Emit a reference.
		return model_core.NewSimplePatchedMessage[TMetadata](
			&model_starlark_pb.Value{
				Kind: &model_starlark_pb.Value_Transition{
					Transition: &model_starlark_pb.Transition{
						Kind: &model_starlark_pb.Transition_Reference_{
							Reference: &model_starlark_pb.Transition_Reference{
								Kind: &model_starlark_pb.Transition_Reference_UserDefined{
									UserDefined: td.Identifier.String(),
								},
							},
						},
					},
				},
			},
		), false, nil
	}

	implementation, needsCode, err := td.implementation.Encode(path, options)
	if err != nil {
		return model_core.PatchedMessage[*model_starlark_pb.Value, TMetadata]{}, false, err
	}
	return model_core.NewPatchedMessage(
		&model_starlark_pb.Value{
			Kind: &model_starlark_pb.Value_Transition{
				Transition: &model_starlark_pb.Transition{
					Kind: &model_starlark_pb.Transition_Definition_{
						Definition: &model_starlark_pb.Transition_Definition{
							Implementation: implementation.Message,
							Inputs:         td.inputs,
							Outputs:        td.outputs,
						},
					},
				},
			},
		},
		implementation.Patcher,
	), needsCode, nil
}

type transitionDefinitionUnpackerInto[TReference any, TMetadata model_core.CloneableReferenceMetadata] struct{}

// NewTransitionDefinitionUnpackerInto is capable of unpacking arguments
// to a Starlark function that are expected to refer to a configuration
// transition. These may either be user defined transitions, or strings
// referring to predefined transitions (i.e., "exec" or "target").
func NewTransitionDefinitionUnpackerInto[TReference any, TMetadata model_core.CloneableReferenceMetadata]() unpack.UnpackerInto[TransitionDefinition[TReference, TMetadata]] {
	return transitionDefinitionUnpackerInto[TReference, TMetadata]{}
}

func (transitionDefinitionUnpackerInto[TReference, TMetadata]) UnpackInto(thread *starlark.Thread, v starlark.Value, dst *TransitionDefinition[TReference, TMetadata]) error {
	switch typedV := v.(type) {
	case starlark.String:
		switch typedV {
		case "exec":
			*dst = NewReferenceTransitionDefinition[TReference, TMetadata](&DefaultExecGroupTransitionReference)
		case "target":
			*dst = NewReferenceTransitionDefinition[TReference, TMetadata](&TargetTransitionReference)
		default:
			return fmt.Errorf("got %#v, want \"exec\" or \"target\"", typedV)
		}
		return nil
	case *Transition[TReference, TMetadata]:
		*dst = typedV.TransitionDefinition
		return nil
	default:
		return fmt.Errorf("got %s, want transition or str", v.Type())
	}
}

func (ui transitionDefinitionUnpackerInto[TReference, TMetadata]) Canonicalize(thread *starlark.Thread, v starlark.Value) (starlark.Value, error) {
	var td TransitionDefinition[TReference, TMetadata]
	if err := ui.UnpackInto(thread, v, &td); err != nil {
		return nil, err
	}
	return NewTransition[TReference, TMetadata](td), nil
}

func (transitionDefinitionUnpackerInto[TReference, TMetadata]) GetConcatenationOperator() syntax.Token {
	return 0
}

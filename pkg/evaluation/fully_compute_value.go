package evaluation

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"sort"

	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	"github.com/buildbarn/bonanza/pkg/storage/dag"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func getKeyString[TReference object.BasicReference](key model_core.TopLevelMessage[proto.Message, TReference]) (string, error) {
	anyKey, err := model_core.MarshalTopLevelAny(key)
	if err != nil {
		return "", err
	}
	marshaledKey, err := model_core.MarshalTopLevelMessage(anyKey)
	return string(marshaledKey), err
}

func appendFormattedKey[TReference object.BasicReference](out []byte, key model_core.Message[proto.Message, TReference]) []byte {
	out, _ = protojson.MarshalOptions{}.MarshalAppend(out, key.Message)
	if degree := key.OutgoingReferences.GetDegree(); degree > 0 {
		out = append(out, " ["...)
		for i := 0; i < degree; i++ {
			if i > 0 {
				out = append(out, ", "...)
			}
			out = base64.RawURLEncoding.AppendEncode(out, key.OutgoingReferences.GetOutgoingReference(i).GetRawReference())
		}
		out = append(out, ']')
	}
	return out
}

type keyState[TReference object.BasicReference, TMetadata any] struct {
	parent       *keyState[TReference, TMetadata]
	key          model_core.Message[proto.Message, TReference]
	next         *keyState[TReference, TMetadata]
	value        valueState[TReference, TMetadata]
	blockedCount int
	blocking     map[*keyState[TReference, TMetadata]]struct{}
	dependencies []string
}

func (ks *keyState[TReference, TMetadata]) markBlocked(ksBlocker *keyState[TReference, TMetadata]) {
	if _, ok := ksBlocker.blocking[ks]; !ok {
		ksBlocker.blocking[ks] = struct{}{}
		ks.blockedCount++
	}
}

func (ks *keyState[TReference, TMetadata]) getKeyType() string {
	return string(ks.key.Message.ProtoReflect().Descriptor().FullName().Parent().Name())
}

func (ks *keyState[TReference, TMetadata]) getDependencyCycle(
	cyclePath *[]*keyState[TReference, TMetadata],
	seen map[*keyState[TReference, TMetadata]]struct{},
	missingDependencies map[*keyState[TReference, TMetadata]][]*keyState[TReference, TMetadata],
) bool {
	pathLength := len(*cyclePath)
	for _, ksDep := range missingDependencies[ks] {
		*cyclePath = append(*cyclePath, ksDep)
		if _, ok := seen[ksDep]; ok {
			return true
		}
		seen[ksDep] = struct{}{}

		if ksDep.getDependencyCycle(cyclePath, seen, missingDependencies) {
			return true
		}

		*cyclePath = (*cyclePath)[:pathLength]
		delete(seen, ksDep)
	}
	return false
}

type valueState[TReference object.BasicReference, TMetadata any] interface {
	compute(ctx context.Context, c Computer[TReference, TMetadata], e *fullyComputingEnvironment[TReference, TMetadata]) error
	getMessageValue() model_core.Message[proto.Message, TReference]
}

type messageValueState[TReference object.BasicReference, TMetadata any] struct {
	value model_core.Message[proto.Message, TReference]
}

func (vs *messageValueState[TReference, TMetadata]) compute(ctx context.Context, c Computer[TReference, TMetadata], e *fullyComputingEnvironment[TReference, TMetadata]) error {
	value, err := c.ComputeMessageValue(ctx, e.keyState.key, e)
	if err != nil {
		return err
	}

	// Value got computed. Write any objects referenced by the
	// values to storage.
	p := e.pool
	localReferences, objectContentsWalkers := value.Patcher.SortAndSetReferences()
	var storedReferences []TReference
	if len(localReferences) > 0 {
		storedReferences, err = p.storeValueChildren(localReferences, objectContentsWalkers)
		if err != nil {
			return err
		}
	}

	vs.value = model_core.NewMessage(value.Message, object.OutgoingReferencesList[TReference](storedReferences))
	return nil
}

func (vs *messageValueState[TReference, TMetadata]) getMessageValue() model_core.Message[proto.Message, TReference] {
	return vs.value
}

type nativeValueState[TReference object.BasicReference, TMetadata any] struct {
	value any
	isSet bool
}

func (vs *nativeValueState[TReference, TMetadata]) compute(ctx context.Context, c Computer[TReference, TMetadata], e *fullyComputingEnvironment[TReference, TMetadata]) error {
	value, err := c.ComputeNativeValue(ctx, e.keyState.key, e)
	if err != nil {
		return err
	}
	vs.value = value
	vs.isSet = true
	return nil
}

func (vs *nativeValueState[TReference, TMetadata]) getMessageValue() model_core.Message[proto.Message, TReference] {
	return model_core.Message[proto.Message, TReference]{}
}

type fullyComputingKeyPool[TReference object.BasicReference, TMetadata any] struct {
	// Constant fields.
	storeValueChildren ValueChildrenStorer[TReference]

	// Variable fields.
	keys                map[string]*keyState[TReference, TMetadata]
	firstPendingKey     *keyState[TReference, TMetadata]
	lastPendingKey      **keyState[TReference, TMetadata]
	lastKeyWithBlockers *keyState[TReference, TMetadata]
	err                 error
}

func (p *fullyComputingKeyPool[TReference, TMetadata]) setError(err error) {
	if p.err == nil {
		p.err = err
	}
}

func (p *fullyComputingKeyPool[TReference, TMetadata]) enqueue(ks *keyState[TReference, TMetadata]) {
	*p.lastPendingKey = ks
	p.lastPendingKey = &ks.next
}

type fullyComputingEnvironment[TReference object.BasicReference, TMetadata any] struct {
	model_core.ObjectManager[TReference, TMetadata]

	pool         *fullyComputingKeyPool[TReference, TMetadata]
	keyState     *keyState[TReference, TMetadata]
	dependencies map[string]struct{}
}

func (e *fullyComputingEnvironment[TReference, TMetadata]) getKeyState(patchedKey model_core.PatchedMessage[proto.Message, dag.ObjectContentsWalker], vs valueState[TReference, TMetadata]) *keyState[TReference, TMetadata] {
	// If the key contains outgoing references, ensure children are
	// written, so that they can be reloaded during evaluation.
	p := e.pool
	topLevelKey, objectContentsWalkers := patchedKey.SortAndSetReferences()
	var storedReferences []TReference
	if outgoingReferences := topLevelKey.OutgoingReferences; outgoingReferences.GetDegree() > 0 {
		var err error
		storedReferences, err = e.pool.storeValueChildren(outgoingReferences, objectContentsWalkers)
		if err != nil {
			p.setError(err)
			return nil
		}
	}

	key := model_core.NewTopLevelMessage(patchedKey.Message, object.OutgoingReferencesList[TReference](storedReferences))
	keyStr, err := getKeyString(key)
	if err != nil {
		p.setError(err)
		return nil
	}
	e.dependencies[keyStr] = struct{}{}
	ks, ok := p.keys[keyStr]
	if !ok {
		ks = &keyState[TReference, TMetadata]{
			parent:   e.keyState,
			key:      key.Decay(),
			value:    vs,
			blocking: map[*keyState[TReference, TMetadata]]struct{}{},
		}
		p.keys[keyStr] = ks
		p.enqueue(ks)
	}
	return ks
}

func (e *fullyComputingEnvironment[TReference, TMetadata]) GetMessageValue(patchedKey model_core.PatchedMessage[proto.Message, dag.ObjectContentsWalker]) model_core.Message[proto.Message, TReference] {
	ks := e.getKeyState(patchedKey, &messageValueState[TReference, TMetadata]{})
	if ks == nil {
		return model_core.Message[proto.Message, TReference]{}
	}
	vs := ks.value.(*messageValueState[TReference, TMetadata])
	if !vs.value.IsSet() {
		e.keyState.markBlocked(ks)
	}
	return vs.value
}

func (e *fullyComputingEnvironment[TReference, TMetadata]) GetNativeValue(patchedKey model_core.PatchedMessage[proto.Message, dag.ObjectContentsWalker]) (any, bool) {
	ks := e.getKeyState(patchedKey, &nativeValueState[TReference, TMetadata]{})
	if ks == nil {
		return nil, false
	}
	vs := ks.value.(*nativeValueState[TReference, TMetadata])
	if !vs.isSet {
		e.keyState.markBlocked(ks)
		return nil, false
	}
	return vs.value, true
}

func (e *fullyComputingEnvironment[TReference, TMetadata]) Store(references []object.LocalReference, walkers []dag.ObjectContentsWalker) object.OutgoingReferences[TReference] {
	panic("TODO")
}

type ValueChildrenStorer[TReference any] func(localReferences object.OutgoingReferences[object.LocalReference], objectContentsWalkers []dag.ObjectContentsWalker) ([]TReference, error)

type Evaluation[TReference any] struct {
	Key          model_core.Message[proto.Message, TReference]
	Value        model_core.Message[proto.Message, TReference]
	Dependencies []model_core.Message[proto.Message, TReference]
}

func FullyComputeValue[TReference object.BasicReference, TMetadata any](
	ctx context.Context,
	c Computer[TReference, TMetadata],
	requestedKey model_core.TopLevelMessage[proto.Message, TReference],
	objectManager model_core.ObjectManager[TReference, TMetadata],
	storeValueChildren ValueChildrenStorer[TReference],
) (
	model_core.Message[proto.Message, TReference],
	iter.Seq[Evaluation[TReference]],
	[]model_core.Message[proto.Message, TReference],
	error,
) {
	keys := map[string]*keyState[TReference, TMetadata]{}
	resultsIterator := func(yield func(Evaluation[TReference]) bool) {
		for _, key := range slices.Sorted(maps.Keys(keys)) {
			ks := keys[key]

			value := ks.value.getMessageValue()
			dependencies := make([]model_core.Message[proto.Message, TReference], 0, len(ks.dependencies))
			sort.Strings(ks.dependencies)
			for _, dependency := range ks.dependencies {
				dependencies = append(dependencies, keys[dependency].key)
			}

			// Omit keys for which there's nothing
			// meaningful to report. Those only blow up the
			// size of the data.
			if !value.IsSet() && len(dependencies) == 0 {
				continue
			}

			if !yield(Evaluation[TReference]{
				Key:          ks.key,
				Value:        value,
				Dependencies: dependencies,
			}) {
				break
			}
		}
	}

	requestedKeyStr, err := getKeyString(requestedKey)
	if err != nil {
		return model_core.Message[proto.Message, TReference]{}, resultsIterator, nil, err
	}
	requestedValueState := &messageValueState[TReference, TMetadata]{}
	requestedKeyState := &keyState[TReference, TMetadata]{
		key:   requestedKey.Decay(),
		value: requestedValueState,
	}
	keys[requestedKeyStr] = requestedKeyState
	p := fullyComputingKeyPool[TReference, TMetadata]{
		storeValueChildren: storeValueChildren,
		keys:               keys,
		firstPendingKey:    requestedKeyState,
		lastPendingKey:     &requestedKeyState.next,
	}

	longestKeyType := 0
	for !requestedValueState.value.IsSet() {
		ks := p.firstPendingKey
		if ks == nil {
			stack := []*keyState[TReference, TMetadata]{requestedKeyState}
			seen := map[*keyState[TReference, TMetadata]]struct{}{
				requestedKeyState: {},
			}
			missingDependencies := map[*keyState[TReference, TMetadata]][]*keyState[TReference, TMetadata]{}
			for _, ks := range keys {
				for ksBlocked := range ks.blocking {
					missingDependencies[ksBlocked] = append(missingDependencies[ksBlocked], ks)
				}
			}
			requestedKeyState.getDependencyCycle(
				&stack,
				seen,
				missingDependencies,
			)

			stackKeys := make([]model_core.Message[proto.Message, TReference], 0, len(stack))
			for _, ks := range stack {
				stackKeys = append(stackKeys, ks.key)
			}
			return model_core.Message[proto.Message, TReference]{}, resultsIterator, stackKeys, errors.New("cyclic evaluation dependency detected")
		}

		p.firstPendingKey = ks.next
		ks.next = nil
		if p.firstPendingKey == nil {
			p.lastPendingKey = &p.firstPendingKey
		}

		// TODO: Add interface for exposing information on
		// what's going on.
		keyType := ks.getKeyType()
		if l := len(keyType); longestKeyType < l {
			longestKeyType = l
		}
		fmt.Printf("\x1b[?7l%-*s  %s\x1b[?7h", longestKeyType, keyType, string(appendFormattedKey(nil, ks.key)))

		e := fullyComputingEnvironment[TReference, TMetadata]{
			ObjectManager: objectManager,
			pool:          &p,
			keyState:      ks,
			dependencies:  map[string]struct{}{},
		}
		err = ks.value.compute(ctx, c, &e)
		ks.dependencies = slices.Collect(maps.Keys(e.dependencies))
		if err != nil {
			if p.err != nil {
				err = p.err
			}
			if !errors.Is(err, ErrMissingDependency) {
				fmt.Printf("\n")

				var stack []model_core.Message[proto.Message, TReference]
				for ksIter := ks; ksIter != nil; ksIter = ksIter.parent {
					stack = append(stack, ksIter.key)
				}
				slices.Reverse(stack)
				return model_core.Message[proto.Message, TReference]{}, resultsIterator, stack, err
			}
			// Value could not be computed, because one of
			// its dependencies hasn't been computed yet.
			if ks.blockedCount == 0 {
				panic("function returned ErrMissingDependency, but no missing dependencies were registered")
			}
			fmt.Printf("\r\x1b[K")
		} else {
			// Successfully computed value. Unblock any keys
			// that were waiting on us and enqueue them if
			// they no longer have any blockers.
			for ksBlocked := range ks.blocking {
				ksBlocked.blockedCount--
				if ksBlocked.blockedCount == 0 {
					p.enqueue(ksBlocked)
				}
			}
			ks.blocking = nil

			fmt.Printf("\n")
		}
	}

	return requestedValueState.value, resultsIterator, nil, nil
}

type ValueChildrenStorerForTesting ValueChildrenStorer[object.LocalReference]

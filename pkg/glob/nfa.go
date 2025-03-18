package glob

import (
	"errors"
	"fmt"
	"math/bits"
	"unicode"
	"unicode/utf8"

	"github.com/buildbarn/bonanza/pkg/encoding/varint"
)

const (
	maxRuneBits = 21

	stateBits = (64 - 21) / 2
	stateMask = 1<<stateBits - 1
)

func init() {
	if bits.Len(unicode.MaxRune) > maxRuneBits {
		panic("maxRuneBits is too small")
	}
}

// endState indicates whether a given state of the nondeterministic
// finite automaton is an end state (i.e., a state in which parsing
// terminates).
type endState int

const (
	endStateNone     endState = 0
	endStatePositive endState = 1
	endStateNegative endState = 2
)

// state keeps track of all properties of a state of the NFA for parsing
// glob patterns.
type state struct {
	// Outgoing edge transitions for wildcards "*" and "**/", and
	// whether the current state is an end state. This field holds
	// the following data:
	//
	// - Bits 0 to 20: If non-zero, the current state has an
	//   outgoing edge for wildcard "*". The index of the target
	//   state is provided.
	//
	// - Bits 21 to 41: If non-zero, the current state has an
	//   outgoing edge for wildcard "**/". The index of the target
	//   state is provided.
	//
	// - Bit 42: Whether this state or any of its successive
	//   states are a positive end state. If not set, attempts to
	//   patch paths may return early.
	//
	// - Bits 43 and 44: Whether the current state is an end state.
	wildcards uint64

	// Outgoing edge transitions for regular characters. These are
	// stored in the form of a linked list, where each state
	// references its sibling. This fields holds the following data:
	//
	// - Bits 0 to 20: If non-zero, the current state has one or more
	//   outgoing references for non-wildcard characters. The index
	//   of the state associated with the first outgoing edge is
	//   provided.
	//
	// - Bits 21 to 41: If non-zero, the parent state has multiple
	//   outgoing references for non-wildcard characters. The index
	//   of the next sibling of the current state is provided.
	//
	// - Bits 42 to 62: The rune associated with the outgoing edge
	//   pointing from the parent state to the current state.
	runes uint64
}

func (s *state) getStarIndex() uint32 {
	return uint32(s.wildcards & stateMask)
}

func (s *state) setStarIndex(i uint32) {
	s.wildcards = s.wildcards&^stateMask | uint64(i)
}

func (s *state) getStarStarSlashIndex() uint32 {
	return uint32(s.wildcards >> stateBits & stateMask)
}

func (s *state) setStarStarSlashIndex(i uint32) {
	s.wildcards = s.wildcards&^(stateMask<<stateBits) | (uint64(i) << stateBits)
}

func (s *state) propagateHasPositiveEndState(states []state) {
	const hasPositiveEndState = 1 << (2 * stateBits)
	if s.getEndState() == endStatePositive ||
		states[s.getStarIndex()].getHasPositiveEndState() ||
		states[s.getStarStarSlashIndex()].getHasPositiveEndState() {
		s.wildcards |= hasPositiveEndState
		return
	}
	for index := s.getFirstRuneIndex(); index != 0; index = states[index].getNextRuneIndex() {
		if states[index].getHasPositiveEndState() {
			s.wildcards |= hasPositiveEndState
			return
		}
	}
}

func (s *state) getHasPositiveEndState() bool {
	return s.wildcards&(1<<(2*stateBits)) != 0
}

func (s *state) getEndState() endState {
	return endState(s.wildcards >> (2*stateBits + 1))
}

func (s *state) setEndState(endState endState) {
	const shift = 2*stateBits + 1
	s.wildcards = s.wildcards&(1<<shift-1) | uint64(endState)<<shift
}

func (s *state) getCurrentRune() rune {
	return rune(s.runes >> (2 * stateBits))
}

func (s *state) initializeRune(r rune, nextRuneIndex uint32) {
	s.runes = (uint64(r) << (2 * stateBits)) | uint64(nextRuneIndex)<<stateBits
}

func (s *state) getFirstRuneIndex() uint32 {
	return uint32(s.runes & stateMask)
}

func (s *state) setFirstRuneIndex(i uint32) {
	s.runes = s.runes&^stateMask | uint64(i)
}

func (s *state) getNextRuneIndex() uint32 {
	return uint32(s.runes >> stateBits & stateMask)
}

func (s *state) setNextRuneIndex(i uint32) {
	s.runes = s.runes&^(stateMask<<stateBits) | (uint64(i) << stateBits)
}

// addState attaches a new state to the NFA that is being constructed.
// The newly created state has no outgoing edges.
func addState(states *[]state) (uint32, error) {
	i := uint32(len(*states))
	if i > stateMask {
		return 0, fmt.Errorf("patterns require more than %d states to represent", stateMask)
	}
	*states = append(*states, state{})
	return i, nil
}

// getOrAddRune returns the index of the state associated with the
// outgoing edge for a non-wildcard character. A new outgoing edge and
// state are created if none exists yet.
func getOrAddRune(states *[]state, currentState uint32, r rune) (uint32, error) {
	// Find the spot at which to insert a new outgoing edge. We want
	// to keep outgoing edges sorted by rune, so that the output
	// remains stable even if patterns are reordered.
	previousState, compareState := uint32(0), (*states)[currentState].getFirstRuneIndex()
	for ; compareState != 0; previousState, compareState = compareState, (*states)[compareState].getNextRuneIndex() {
		if cr := (*states)[compareState].getCurrentRune(); cr == r {
			// Found an existing outgoing edge.
			return compareState, nil
		} else if cr > r {
			break
		}
	}

	// No outgoing edge found. Create a new state and insert an
	// outgoing edge in the right spot.
	i, err := addState(states)
	if err != nil {
		return 0, err
	}
	if previousState == 0 {
		(*states)[currentState].setFirstRuneIndex(i)
	} else {
		(*states)[previousState].setNextRuneIndex(i)
	}
	(*states)[i].initializeRune(r, compareState)
	return i, nil
}

// getOrAddRune returns the index of the state associated with the
// outgoing edge for wildcard "*". A new outgoing edge and state are
// created if none exists yet.
func getOrAddStar(states *[]state, currentState uint32) (uint32, error) {
	if i := (*states)[currentState].getStarIndex(); i != 0 {
		return i, nil
	}
	i, err := addState(states)
	if err != nil {
		return 0, err
	}
	(*states)[currentState].setStarIndex(i)
	return i, nil
}

// getOrAddRune returns the index of the state associated with the
// outgoing edge for wildcard "**/". A new outgoing edge and state are
// created if none exists yet.
func getOrAddStarStarSlash(states *[]state, currentState uint32) (uint32, error) {
	if i := (*states)[currentState].getStarStarSlashIndex(); i != 0 {
		return i, nil
	}
	i, err := addState(states)
	if err != nil {
		return 0, err
	}
	(*states)[currentState].setStarStarSlashIndex(i)
	return i, nil
}

func addPattern(states *[]state, pattern string, endState endState) error {
	// Walk over the pattern and add states to the NFA for each
	// construct we encounter.
	gotRunes := false
	gotStarStarSlash := false
	gotStars := 0
	previousDirectoryEndState := uint32(0)
	currentState := uint32(0)
	var err error
	for _, r := range pattern {
		if r == '*' {
			switch gotStars {
			case 1:
				if gotRunes {
					return errors.New("\"**\" can not be placed inside a component")
				}
			case 2:
				return errors.New("\"***\" has no special meaning")
			}
			gotStars++
		} else {
			if r == '/' {
				if gotStars == 2 {
					if gotStarStarSlash {
						return errors.New("redundant \"**\"")
					}

					currentState, err = getOrAddStarStarSlash(states, currentState)
					if err != nil {
						return err
					}
					gotStarStarSlash = true
				} else {
					switch gotStars {
					case 0:
						if !gotRunes {
							return errors.New("pathname components cannot be empty")
						}
					case 1:
						currentState, err = getOrAddStar(states, currentState)
						if err != nil {
							return err
						}
					default:
						panic("unexpected number of stars")
					}

					previousDirectoryEndState = currentState
					currentState, err = getOrAddRune(states, currentState, '/')
					if err != nil {
						return err
					}
					gotStarStarSlash = false
				}
				gotRunes = false
			} else {
				switch gotStars {
				case 1:
					currentState, err = getOrAddStar(states, currentState)
					if err != nil {
						return err
					}
				case 2:
					return errors.New("\"**\" can not be placed inside a component")
				}
				currentState, err = getOrAddRune(states, currentState, r)
				if err != nil {
					return err
				}
				gotRunes = true
			}
			gotStars = 0
		}
	}

	// Process any trailing stars.
	switch gotStars {
	case 0:
		if !gotRunes {
			return errors.New("pathname components cannot be empty")
		}
	case 1:
		currentState, err = getOrAddStar(states, currentState)
		if err != nil {
			return err
		}
	case 2:
		if gotStarStarSlash {
			return errors.New("redundant \"**\"")
		}

		// Convert any trailing "**" to "**/*". For consistency
		// with Bazel, a bare "**" does not match the root
		// directory, while "foo/**" does match "foo".
		if previousDirectoryEndState > 0 {
			(*states)[previousDirectoryEndState].setEndState(endState)
		}
		currentState, err = getOrAddStarStarSlash(states, currentState)
		if err != nil {
			return err
		}
		currentState, err = getOrAddStar(states, currentState)
		if err != nil {
			return err
		}
	default:
		panic("unexpected number of stars")
	}
	(*states)[currentState].setEndState(endState)
	return nil
}

// NFA is a nondeterministic finite automaton of a set of glob patterns.
// This automaton can be used to perform matching of glob patterns
// against paths.
type NFA struct {
	states []state
}

// NewNFAFromPatterns compiles a set of glob patterns into a
// nondeterministic finite automaton (NFA) that is capable of matching
// paths.
func NewNFAFromPatterns(includes, excludes []string) (*NFA, error) {
	states := []state{{}}
	for _, pattern := range includes {
		if err := addPattern(&states, pattern, endStatePositive); err != nil {
			return nil, fmt.Errorf("invalid \"includes\" pattern %#v: %w", pattern, err)
		}
	}
	for _, pattern := range excludes {
		if err := addPattern(&states, pattern, endStateNegative); err != nil {
			return nil, fmt.Errorf("invalid \"excludes\" pattern %#v: %w", pattern, err)
		}
	}

	for currentState := len(states) - 1; currentState > 0; currentState-- {
		states[currentState].propagateHasPositiveEndState(states)
	}
	return &NFA{states: states}, nil
}

// NewNFAFromBytes reobtains a nondeterministic finite automaton (NFA)
// that is capable of matching paths from a previously generated binary
// representation.
func NewNFAFromBytes(b []byte) (*NFA, error) {
	if len(b) > stateMask {
		return nil, fmt.Errorf("NFA may require more than %d states to represent", stateMask)
	}

	// Perform a forward pass to ingest all states.
	states := []state{{}}
	for currentState := 0; currentState < len(states); currentState++ {
		// Consume the tag of the state.
		tag, n := varint.ConsumeForward[uint](b)
		if n < 0 {
			return nil, fmt.Errorf("got end of file after %d states, while at least %d states were expected", currentState, len(states))
		}
		b = b[n:]

		// Create new states for wildcards "*" and "**/".
		states[currentState].setEndState(endState(tag & 0x3))
		if tag&(1<<2) != 0 {
			states[currentState].setStarIndex(uint32(len(states)))
			states = append(states, state{})
		}
		if tag&(1<<3) != 0 {
			states[currentState].setStarStarSlashIndex(uint32(len(states)))
			states = append(states, state{})
		}

		// Create new states for regular characters. Join them
		// together using a linked list.
		if runesCount := tag >> 4; runesCount > 0 {
			states[currentState].setFirstRuneIndex(uint32(len(states)))
			for {
				r, n := utf8.DecodeRune(b)
				if r == utf8.RuneError {
					return nil, fmt.Errorf("invalid rune in state %d", currentState)
				}
				b = b[n:]

				states = append(states, state{})
				runesCount--
				if runesCount == 0 {
					states[len(states)-1].initializeRune(r, 0)
					break
				}
				states[len(states)-1].initializeRune(r, uint32(len(states)))
			}
		}
	}

	// Perform a backward pass to propagate state properties.
	for currentState := len(states) - 1; currentState > 0; currentState-- {
		states[currentState].propagateHasPositiveEndState(states)
	}
	return &NFA{states: states}, nil
}

// Bytes returns a copy of the nondeterministic finite automaton in
// binary form, allowing it to be stored or transmitted.
func (nfa *NFA) Bytes() []byte {
	var out []byte
	statesToEmit := []uint32{0}
	for len(statesToEmit) > 0 {
		s := &nfa.states[statesToEmit[0]]
		statesToEmit = statesToEmit[1:]

		// Emit a tag for the state, indicating whether paths
		// that terminate in the state are part of the results,
		// whether the state is followed by a "*" or "**", and
		// the number of outgoing edges for runes.
		tag := s.getEndState()
		if i := s.getStarIndex(); i != 0 {
			tag |= 1 << 2
			statesToEmit = append(statesToEmit, i)
		}
		if i := s.getStarStarSlashIndex(); i != 0 {
			tag |= 1 << 3
			statesToEmit = append(statesToEmit, i)
		}
		for i := s.getFirstRuneIndex(); i != 0; i = nfa.states[i].getNextRuneIndex() {
			tag += 1 << 4
			statesToEmit = append(statesToEmit, i)
		}
		out = varint.AppendForward(out, tag)

		// Emit the runes of the outgoing edges for this state.
		for i := s.getFirstRuneIndex(); i != 0; i = nfa.states[i].getNextRuneIndex() {
			out = utf8.AppendRune(out, nfa.states[i].getCurrentRune())
		}
	}
	return out
}

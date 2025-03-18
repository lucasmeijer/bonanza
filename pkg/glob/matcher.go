package glob

// statesSet is a set type for states. It is used by the matching
// algorithm when computing the set of currently active states and
// wildcards.
type statesSet struct {
	states              map[uint32]struct{}
	hasPositiveEndState bool
}

// Add a state to the set.
func (s *statesSet) add(states []state, index uint32) {
	if s.states == nil {
		s.states = map[uint32]struct{}{}
	}
	s.states[index] = struct{}{}
	s.hasPositiveEndState = s.hasPositiveEndState || states[index].getHasPositiveEndState()
}

// Clear the set of states.
func (s *statesSet) clear() {
	clear(s.states)
	s.hasPositiveEndState = false
}

// copyFrom copies the set of currently active states from another
// instance.
func (s *statesSet) copyFrom(s2 *statesSet) {
	if s.states == nil {
		s.states = make(map[uint32]struct{}, len(s2.states))
	} else {
		clear(s.states)
	}
	for index := range s2.states {
		s.states[index] = struct{}{}
	}
	s.hasPositiveEndState = s2.hasPositiveEndState
}

// Matcher of globs. Zero initialized instances correspond to an empty
// state machine that does not match any strings.
type Matcher struct {
	states []state

	currentStates          statesSet
	currentStars           statesSet
	currentStarStarSlashes statesSet
	previousStatesList     []uint32
}

// Initialize a matcher by letting it point to the initial state of a
// provided nondeterministic finite automaton (NFA).
func (m *Matcher) Initialize(nfa *NFA) {
	m.states = nfa.states
	m.currentStates.clear()
	m.currentStars.clear()
	m.currentStarStarSlashes.clear()
	m.expandState3(0)
}

// CopyFrom overwrites the state of the matcher with that of another
// matcher. This can be used branch the matching process, or reset it to
// a previous state.
//
// This function is implemented so that it reuses previously allocated
// memory. It is therefore recommended to reuse instances of Matcher as
// much as possible.
func (m *Matcher) CopyFrom(m2 *Matcher) {
	m.states = m2.states
	m.currentStates.copyFrom(&m2.currentStates)
	m.currentStars.copyFrom(&m2.currentStars)
	m.currentStarStarSlashes.copyFrom(&m2.currentStarStarSlashes)
}

// expandState1 adds a state to the set of currently active states.
func (m *Matcher) expandState1(index uint32) {
	m.currentStates.add(m.states, index)
}

// expandState1 adds a state to the set of currently active states. If
// it contains a "*" wildcard, it registers it as well.
func (m *Matcher) expandState2(index1 uint32) {
	m.expandState1(index1)
	if index2 := m.states[index1].getStarIndex(); index2 != 0 {
		m.currentStars.add(m.states, index2)
		m.expandState1(index2)
	}
}

// expandState1 adds a state to the set of currently active states. If
// it contains a "*" or "**/" wildcard, it registers it as well.
func (m *Matcher) expandState3(index1 uint32) {
	m.expandState2(index1)
	if index2 := m.states[index1].getStarStarSlashIndex(); index2 != 0 {
		m.currentStarStarSlashes.add(m.states, index2)
		m.expandState2(index2)
	}
}

// WriteRune matches a single character of pathname strings against the
// patterns encoded in the NFA. Upon completion, it returns whether
// there are still any reachable states that would allow matching to
// succeed. When false is returned, the caller should terminate the
// matching process immediately.
func (m *Matcher) WriteRune(r rune) bool {
	// Clear the set of current states, as we're going to repopulate
	// it. Do save the previous states in a temporary slice, because
	// we still need to follow their outgoing edges.
	for index := range m.currentStates.states {
		m.previousStatesList = append(m.previousStatesList, index)
	}
	m.currentStates.clear()

	if r == '/' {
		// A slash should cause all current "*" wildcards to
		// terminate. However, it should cause "**/" wildcards
		// to activate.
		m.currentStars.clear()
		for index := range m.currentStarStarSlashes.states {
			m.expandState3(index)
		}
	} else {
		// A non-slash character should cause all current "*"
		// wildcards to activate.
		for index := range m.currentStars.states {
			m.expandState3(index)
		}
	}

	// Perform matching of regular characters.
	for _, index1 := range m.previousStatesList {
		for index2 := m.states[index1].getFirstRuneIndex(); index2 != 0; index2 = m.states[index2].getNextRuneIndex() {
			cr := m.states[index2].getCurrentRune()
			if r == cr {
				m.expandState3(index2)
			} else if r < cr {
				// Runes are listed in sorted order.
				break
			}
		}
	}

	// Prune the list of previous states. We keep it around to
	// prevent repeated memory allocation.
	m.previousStatesList = m.previousStatesList[:0]

	return m.currentStates.hasPositiveEndState ||
		m.currentStars.hasPositiveEndState ||
		m.currentStarStarSlashes.hasPositiveEndState
}

// IsMatch returns true if the currently parsed input corresponds to a match.
func (m *Matcher) IsMatch() bool {
	maxEndState := endStateNone
	for index := range m.currentStates.states {
		if endState := m.states[index].getEndState(); maxEndState < endState {
			maxEndState = endState
		}
	}
	return maxEndState == endStatePositive
}

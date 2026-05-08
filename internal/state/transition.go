package state

import "errors"

// ErrIllegalTransition is returned when Apply is called with a (from, event) pair
// that is not defined in the state transition table.
var ErrIllegalTransition = errors.New("illegal state transition")

// transitionTable maps each source Status to the set of legal (event → target) pairs.
// Every cell corresponds to a row in PRD §5.1.
var transitionTable = map[Status]map[Event]Status{
	Queued: {
		Pickup: Running,
	},
	Pending: {
		Pickup: Running,
		Kill:   Killed,
	},
	Running: {
		ConstraintsPassed: Ready,
		Block:             Blocked,
		Fail:              Failed,
		Kill:              Killed,
	},
	Blocked: {
		Retry: Running,
		Kill:  Killed,
	},
	Ready: {
		Ship:  Shipped,
		Block: Blocked,
		Kill:  Killed,
	},
	Failed: {
		Retry: Running,
		Kill:  Killed,
	},
	Shipped: {},
	Killed:  {},
}

// Apply drives the state machine: given a source status and an event it returns
// the resulting status. Apply is a pure function — no side effects, no I/O,
// no global mutation.
func Apply(from Status, evt Event) (Status, error) {
	events, ok := transitionTable[from]
	if !ok {
		return "", ErrIllegalTransition
	}
	to, ok := events[evt]
	if !ok {
		return "", ErrIllegalTransition
	}
	return to, nil
}

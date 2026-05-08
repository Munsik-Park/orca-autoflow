package state_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/orca-cli/orca/internal/state"
)

// TestApply_LegalTransitions verifies every legal cell in the PRD §5.1 transition table.
// Table: from → event → expected-to
func TestApply_LegalTransitions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from     state.Status
		evt      state.Event
		expected state.Status
	}{
		// Queued
		{state.Queued, state.Pickup, state.Running},

		// Pending (awaiting prerequisites before execution)
		{state.Pending, state.Pickup, state.Running},
		{state.Pending, state.Kill, state.Killed},

		// Running
		{state.Running, state.ConstraintsPassed, state.Ready},
		{state.Running, state.Block, state.Blocked},
		{state.Running, state.Fail, state.Failed},
		{state.Running, state.Kill, state.Killed},

		// Blocked
		{state.Blocked, state.Retry, state.Running},
		{state.Blocked, state.Kill, state.Killed},

		// Ready
		{state.Ready, state.Ship, state.Shipped},
		{state.Ready, state.Block, state.Blocked},
		{state.Ready, state.Kill, state.Killed},

		// Failed
		{state.Failed, state.Retry, state.Running},
		{state.Failed, state.Kill, state.Killed},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.from)+"_"+string(tc.evt), func(t *testing.T) {
			t.Parallel()
			got, err := state.Apply(tc.from, tc.evt)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// TestApply_IllegalTransitions verifies that at least one illegal event per origin
// state returns ErrIllegalTransition.
func TestApply_IllegalTransitions(t *testing.T) {
	t.Parallel()

	// One illegal event per source state.
	cases := []struct {
		from state.Status
		evt  state.Event
	}{
		{state.Queued, state.Ship},
		{state.Pending, state.Ship},
		{state.Running, state.Pickup},
		{state.Blocked, state.Ship},
		{state.Ready, state.Pickup},
		{state.Failed, state.Ship},
		{state.Shipped, state.Pickup},
		{state.Killed, state.Retry},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.from)+"_"+string(tc.evt)+"_illegal", func(t *testing.T) {
			t.Parallel()
			got, err := state.Apply(tc.from, tc.evt)
			require.Error(t, err)
			assert.True(t, errors.Is(err, state.ErrIllegalTransition), "expected ErrIllegalTransition, got %v", err)
			assert.Equal(t, state.Status(""), got, "zero-value Status expected on error")
		})
	}
}

// TestApply_UnknownStatus verifies that an unknown status is rejected.
func TestApply_UnknownStatus(t *testing.T) {
	t.Parallel()
	_, err := state.Apply(state.Status("invalid"), state.Pickup)
	require.Error(t, err)
	assert.True(t, errors.Is(err, state.ErrIllegalTransition))
}

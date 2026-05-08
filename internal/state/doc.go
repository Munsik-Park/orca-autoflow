// Package state defines the run state machine for orca.
// It exposes a pure Apply function that drives status transitions with no
// side effects, no I/O, and no external dependencies.
package state

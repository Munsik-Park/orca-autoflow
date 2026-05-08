package state

// Status represents the lifecycle state of a run.
type Status string

const (
	// Queued means the run is waiting to be picked up by a free adapter slot.
	Queued Status = "queued"
	// Pending means the run is waiting on prerequisite runs to complete.
	Pending Status = "pending"
	// Running means a coding agent is actively working on the run.
	Running Status = "running"
	// Blocked means the run hit a constraint failure and is paused.
	Blocked Status = "blocked"
	// Ready means the run passed all constraints and is ready for human review.
	Ready Status = "ready"
	// Failed means the run encountered a non-recoverable error.
	Failed Status = "failed"
	// Shipped means the run was merged and the worktree archived.
	Shipped Status = "shipped"
	// Killed means the run was explicitly stopped by the user.
	Killed Status = "killed"
)

// Event represents a trigger that can drive a state transition.
type Event string

const (
	// Pickup fires when an adapter slot becomes available and picks up a queued run.
	Pickup Event = "pickup"
	// ConstraintsPassed fires when all constraints evaluate to pass.
	ConstraintsPassed Event = "constraints_passed"
	// Block fires when a constraint evaluation fails.
	Block Event = "block"
	// Fail fires when the adapter reports a non-recoverable execution error.
	Fail Event = "fail"
	// Kill fires when the user explicitly stops a run.
	Kill Event = "kill"
	// Ship fires when the user merges the run's branch.
	Ship Event = "ship"
	// Retry fires when the user requests re-execution of a blocked or failed run.
	Retry Event = "retry"
)

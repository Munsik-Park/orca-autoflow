package adapter

import "context"

// Capabilities describes what a coding agent adapter supports.
type Capabilities struct {
	// CanReview indicates the adapter can perform code review passes.
	CanReview bool
	// CanDiff indicates the adapter can produce structured diffs.
	CanDiff bool
	// MaxContextTokens is the maximum context size in tokens (0 = unlimited/unknown).
	MaxContextTokens int
}

// LaunchRequest carries all parameters needed to start a coding agent run.
type LaunchRequest struct {
	// RunID is the unique identifier of the run being launched.
	RunID string
	// WorktreePath is the absolute path of the git worktree for this run.
	WorktreePath string
	// Prompt is the task description sent to the agent.
	Prompt string
	// ContextFiles lists additional files to include in the agent's context.
	ContextFiles []string
}

// Handle represents a running agent process that can be waited on.
type Handle struct {
	// PID is the OS process ID of the agent process, if applicable.
	PID int
	// SessionID is an opaque token the adapter uses to resume or query the run.
	SessionID string
}

// Result is the outcome of a completed agent run.
type Result struct {
	// Status is the terminal state reported by the agent.
	Status Status
	// Decision is the agent's conclusion (e.g. merged, abandoned, needs-review).
	Decision Decision
	// Error contains error details when Status is StatusError.
	Error *Error
}

// Status describes the terminal outcome of an adapter run.
type Status string

const (
	// StatusSuccess means the agent completed the task successfully.
	StatusSuccess Status = "success"
	// StatusError means the agent encountered a non-recoverable error.
	StatusError Status = "error"
	// StatusKilled means the run was explicitly terminated.
	StatusKilled Status = "killed"
)

// Decision records the adapter's final recommendation after completing a run.
type Decision string

const (
	// DecisionMerge means the agent recommends merging the changes.
	DecisionMerge Decision = "merge"
	// DecisionAbandon means the agent recommends abandoning the run.
	DecisionAbandon Decision = "abandon"
	// DecisionNeedsReview means the agent completed but wants human review.
	DecisionNeedsReview Decision = "needs_review"
)

// Error carries structured error information from a failed adapter run.
type Error struct {
	// Code is a machine-readable error identifier.
	Code string
	// Message is a human-readable description of the error.
	Message string
}

// Config holds the runtime configuration passed to a Factory when
// constructing a new Adapter instance.
type Config struct {
	// Name is the registered adapter name (e.g. "claude-code", "codex").
	Name string
	// Extra holds adapter-specific configuration key-value pairs.
	Extra map[string]string
}

// Adapter is the hexagonal port that every coding agent adapter must implement.
// Concrete implementations live outside this package; they depend on adapter, not vice versa.
type Adapter interface {
	// Name returns the canonical adapter identifier (e.g. "claude-code").
	Name() string
	// Capabilities returns the feature set supported by this adapter.
	Capabilities() Capabilities
	// Validate checks that the adapter's environment is correctly configured
	// (e.g. required binaries present, credentials valid) without starting a run.
	Validate(ctx context.Context) error
	// Launch starts a coding agent run and returns a Handle for tracking it.
	Launch(ctx context.Context, req LaunchRequest) (Handle, error)
}

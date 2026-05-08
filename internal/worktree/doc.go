// Package worktree manages git worktrees for orca coding agent runs.
// Each active run lives in its own isolated worktree under <repo>/.orca/runs/<runID>/.
// When a run completes it is archived to .orca/runs/_shipped/<runID>/ or
// .orca/runs/_killed/<runID>/ and removed from git's worktree registry.
package worktree

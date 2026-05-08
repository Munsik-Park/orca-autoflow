package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "orca",
	Short: "orca — coding agent orchestrator",
	Long: `orca orchestrates multiple coding agents running in parallel git worktrees.
It manages pod lifecycle, run state transitions, constraints, and agent adapters.`,
}

// Execute runs the root command. It is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

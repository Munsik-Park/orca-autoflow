package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version, Commit, and Date are injected at build time via -ldflags.
// Example: -ldflags "-X github.com/orca-cli/orca/internal/cli.Version=v0.1.0"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print orca version information",
	Run: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "orca %s (commit: %s, built: %s)\n", Version, Commit, Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

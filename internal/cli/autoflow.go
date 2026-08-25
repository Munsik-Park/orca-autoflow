package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Munsik-Park/orca-autoflow/internal/autoflow"
	"github.com/spf13/cobra"
)

var autoflowCmd = &cobra.Command{
	Use:   "autoflow",
	Short: "Run AutoFlow phases through Orca adapters",
}

type autoflowStepOptions struct {
	target           string
	issue            int
	phase            string
	adapter          string
	model            string
	prompt           string
	promptFile       string
	runner           string
	allowClosedIssue bool
	dryRun           bool
}

func newAutoflowStepCmd() *cobra.Command {
	opts := &autoflowStepOptions{}
	cmd := &cobra.Command{
		Use:   "step",
		Short: "Run one artifact-gated AutoFlow phase",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAutoflowStep(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.target, "target", ".", "target project root")
	cmd.Flags().IntVar(&opts.issue, "issue", 0, "GitHub issue number")
	cmd.Flags().StringVar(&opts.phase, "phase", "", "AutoFlow phase to run")
	cmd.Flags().StringVar(&opts.adapter, "adapter", "codex", "agent adapter")
	cmd.Flags().StringVar(&opts.model, "model", "", "adapter model identifier; omit to use the adapter default")
	cmd.Flags().StringVar(&opts.prompt, "prompt", "", "task prompt")
	cmd.Flags().StringVar(&opts.promptFile, "prompt-file", "", "path to task prompt file")
	cmd.Flags().StringVar(&opts.runner, "runner", "", "adapter runner path")
	cmd.Flags().BoolVar(&opts.allowClosedIssue, "allow-closed-issue", false, "allow running against a closed GitHub issue")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "validate and print the adapter command without running it")
	return cmd
}

func runAutoflowStep(cmd *cobra.Command, opts *autoflowStepOptions) error {
	if opts.issue <= 0 {
		return fmt.Errorf("--issue must be a positive integer")
	}
	if strings.TrimSpace(opts.phase) == "" {
		return fmt.Errorf("--phase is required")
	}
	if opts.adapter != "codex" {
		return fmt.Errorf("unsupported adapter %q: only codex is available for autoflow step", opts.adapter)
	}
	if opts.prompt != "" && opts.promptFile != "" {
		return fmt.Errorf("choose either --prompt or --prompt-file, not both")
	}

	target, err := filepath.Abs(opts.target)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := verifyIssueOpen(ctx, target, opts.issue, opts.allowClosedIssue); err != nil {
		return err
	}
	spec, err := autoflow.LookupPhase(opts.phase)
	if err != nil {
		return err
	}

	missingInputs := missingPaths(autoflow.RenderPaths(target, opts.issue, spec.Inputs))
	if len(missingInputs) > 0 {
		return fmt.Errorf("missing required input artifact(s): %s", strings.Join(relativizeAll(target, missingInputs), ", "))
	}

	runner := opts.runner
	if runner == "" {
		runner = filepath.Join(target, "scripts", "orca", "codex-agent.sh")
	}
	if _, err := os.Stat(runner); err != nil {
		return fmt.Errorf("codex adapter runner not found at %s: %w", runner, err)
	}

	argv := []string{runner, "--target", target, "--phase", spec.Name}
	if opts.model != "" {
		argv = append(argv, "--model", opts.model)
	}
	if opts.promptFile != "" {
		argv = append(argv, "--prompt-file", opts.promptFile)
	} else if opts.prompt != "" {
		argv = append(argv, "--prompt", opts.prompt)
	}

	if opts.dryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "phase: %s\nagent_type: %s\nrunner: %s\n", spec.Name, spec.AgentType, runner)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "command: %s\n", shellJoin(argv))
		return nil
	}

	execCmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	execCmd.Dir = target
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	if opts.prompt == "" && opts.promptFile == "" {
		execCmd.Stdin = cmd.InOrStdin()
	}
	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("run %s adapter: %w", opts.adapter, err)
	}

	outputs := autoflow.RenderPaths(target, opts.issue, spec.Outputs)
	missingOutputs := missingPaths(outputs)
	if len(missingOutputs) > 0 {
		return fmt.Errorf("adapter completed but required output artifact(s) are missing: %s", strings.Join(relativizeAll(target, missingOutputs), ", "))
	}

	state := autoflow.State{
		Issue:              opts.issue,
		Active:             true,
		Phase:              spec.Next,
		LastCompletedPhase: spec.Name,
		Adapter:            opts.adapter,
		Model:              opts.model,
		Artifacts:          artifactMap(target, opts.issue, spec),
	}
	if err := autoflow.SaveState(target, state); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "completed phase %s for issue #%d; next phase: %s\n", spec.Name, opts.issue, spec.Next)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "state: %s\n", autoflow.StatePath(target, opts.issue))
	return nil
}

func missingPaths(paths []string) []string {
	var missing []string
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, path)
		}
	}
	return missing
}

func relativizeAll(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err == nil && !strings.HasPrefix(rel, "..") {
			out = append(out, filepath.ToSlash(rel))
		} else {
			out = append(out, path)
		}
	}
	return out
}

func artifactMap(target string, issue int, spec autoflow.PhaseSpec) map[string]string {
	artifacts := map[string]string{}
	for _, path := range append(spec.Inputs, spec.Outputs...) {
		rendered := autoflow.RenderPaths(target, issue, []string{path})[0]
		key := filepath.Base(rendered)
		key = strings.TrimSuffix(key, filepath.Ext(key))
		artifacts[key] = relativizeAll(target, []string{rendered})[0]
	}
	return artifacts
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			quoted = append(quoted, "''")
			continue
		}
		if strings.IndexFunc(arg, func(r rune) bool {
			return !(r == '/' || r == '-' || r == '_' || r == '.' || r == ':' || r == '=' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
		}) == -1 {
			quoted = append(quoted, arg)
			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
	}
	return strings.Join(quoted, " ")
}

func verifyIssueOpen(ctx context.Context, target string, issue int, allowClosed bool) error {
	if allowClosed {
		return nil
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return nil
	}
	if err := exec.CommandContext(ctx, "git", "-C", target, "remote", "get-url", "origin").Run(); err != nil {
		return nil
	}
	check := exec.CommandContext(ctx, "gh", "issue", "view", fmt.Sprintf("%d", issue), "--json", "state", "-q", ".state")
	check.Dir = target
	out, err := check.Output()
	if err != nil {
		return fmt.Errorf("could not verify GitHub issue #%d state with gh: %w", issue, err)
	}
	state := strings.TrimSpace(string(out))
	if state != "OPEN" {
		return fmt.Errorf("issue #%d is %s; refusing to run AutoFlow step on a closed/non-open issue (use --allow-closed-issue only for an intentional local replay)", issue, state)
	}
	return nil
}

func init() {
	autoflowCmd.AddCommand(newAutoflowStepCmd())
	rootCmd.AddCommand(autoflowCmd)
}

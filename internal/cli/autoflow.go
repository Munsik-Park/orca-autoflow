package cli

import (
	"context"
	"fmt"
	"io"
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
	codexBin         string
	profile          string
	sandbox          string
	networkAccess    bool
	outputLast       string
	prompt           string
	promptFile       string
	runner           string
	printPrompt      bool
	allowClosedIssue bool
	dryRun           bool
}

type autoflowInitOptions struct {
	target           string
	issue            int
	includeGitignore bool
	dryRun           bool
}

func newAutoflowInitCmd() *cobra.Command {
	opts := &autoflowInitOptions{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create AutoFlow templates in a target repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAutoflowInit(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.target, "target", ".", "target project root")
	cmd.Flags().IntVar(&opts.issue, "issue", 0, "GitHub issue number")
	cmd.Flags().BoolVar(&opts.includeGitignore, "gitignore", false, "add Orca local state files to .gitignore")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "print files that would be created without writing them")
	return cmd
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
	cmd.Flags().StringVar(&opts.codexBin, "codex-bin", "", "Codex executable for the built-in codex adapter")
	cmd.Flags().StringVar(&opts.profile, "profile", "", "Codex profile for the built-in codex adapter")
	cmd.Flags().StringVar(&opts.sandbox, "sandbox", "workspace-write", "Codex sandbox: read-only, workspace-write, danger-full-access")
	cmd.Flags().BoolVar(&opts.networkAccess, "network", false, "allow network access inside the Codex sandbox")
	cmd.Flags().StringVar(&opts.outputLast, "output", "", "write Codex's last message to this file")
	cmd.Flags().StringVar(&opts.prompt, "prompt", "", "task prompt")
	cmd.Flags().StringVar(&opts.promptFile, "prompt-file", "", "path to task prompt file")
	cmd.Flags().StringVar(&opts.runner, "runner", "", "external adapter runner path; default is the built-in codex adapter")
	cmd.Flags().BoolVar(&opts.printPrompt, "print-prompt", false, "print the composed built-in adapter prompt without running Codex")
	cmd.Flags().BoolVar(&opts.allowClosedIssue, "allow-closed-issue", false, "allow running against a closed GitHub issue")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "validate and print the adapter command without running it")
	return cmd
}

func runAutoflowInit(cmd *cobra.Command, opts *autoflowInitOptions) error {
	if opts.issue <= 0 {
		return fmt.Errorf("--issue must be a positive integer")
	}
	target, err := filepath.Abs(opts.target)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}

	scaffoldOpts := autoflow.ScaffoldOptions{
		TargetRoot:       target,
		Issue:            opts.issue,
		IncludeGitignore: opts.includeGitignore,
	}
	planned, err := autoflow.PlanScaffold(scaffoldOpts)
	if err != nil {
		return err
	}
	if opts.dryRun {
		printScaffoldResult(cmd, planned, "would create", "exists")
		printGitignoreAdvice(cmd, opts.includeGitignore)
		return nil
	}

	created, err := autoflow.CreateScaffold(scaffoldOpts)
	if err != nil {
		return err
	}
	printScaffoldResult(cmd, created, "create", "exists")
	printGitignoreAdvice(cmd, opts.includeGitignore)
	return nil
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
	sandbox, err := normalizeSandbox(opts.sandbox)
	if err != nil {
		return err
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

	if opts.runner != "" {
		err = runExternalAutoflowAdapter(ctx, cmd, opts, target, spec, sandbox)
	} else {
		err = runBuiltInCodexAdapter(ctx, cmd, opts, target, spec, sandbox)
	}
	if err != nil {
		return err
	}
	if opts.dryRun || opts.printPrompt {
		return nil
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

func runBuiltInCodexAdapter(ctx context.Context, cmd *cobra.Command, opts *autoflowStepOptions, target string, spec autoflow.PhaseSpec, sandbox string) error {
	roleContract, roleSource, err := autoflow.RoleContract(target, spec.AgentType)
	if err != nil {
		return err
	}

	argv := codexAdapterArgs(codexExecutable(opts), target, opts, sandbox)
	if opts.dryRun && !opts.printPrompt {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "phase: %s\nagent_type: %s\nadapter: built-in codex\nrole_contract: %s\n", spec.Name, spec.AgentType, roleSource)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "command: %s\n", shellJoin(argv))
		return nil
	}

	taskPrompt, err := readTaskPrompt(cmd, opts, true)
	if err != nil {
		return err
	}
	composedPrompt := autoflow.ComposePrompt(autoflow.PromptRequest{
		Issue:        opts.issue,
		TargetRoot:   target,
		Phase:        spec,
		RoleContract: roleContract,
		RoleSource:   roleSource,
		TaskPrompt:   taskPrompt,
	})

	if opts.printPrompt {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), composedPrompt)
		return nil
	}

	if _, err := exec.LookPath(argv[0]); err != nil {
		return fmt.Errorf("codex executable %q is not on PATH: %w", argv[0], err)
	}
	execCmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	execCmd.Dir = target
	execCmd.Stdin = strings.NewReader(composedPrompt)
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("run codex adapter: %w", err)
	}
	return nil
}

func runExternalAutoflowAdapter(ctx context.Context, cmd *cobra.Command, opts *autoflowStepOptions, target string, spec autoflow.PhaseSpec, sandbox string) error {
	if _, err := os.Stat(opts.runner); err != nil {
		return fmt.Errorf("codex adapter runner not found at %s: %w", opts.runner, err)
	}
	argv := externalAdapterArgs(opts.runner, target, opts, spec, sandbox)

	if opts.dryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "phase: %s\nagent_type: %s\nrunner: %s\n", spec.Name, spec.AgentType, opts.runner)
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
	return nil
}

func codexExecutable(opts *autoflowStepOptions) string {
	if opts.codexBin != "" {
		return opts.codexBin
	}
	if env := os.Getenv("CODEX_BIN"); env != "" {
		return env
	}
	return "codex"
}

func codexAdapterArgs(codexBin string, target string, opts *autoflowStepOptions, sandbox string) []string {
	argv := []string{
		codexBin,
		"exec",
		"-C", target,
		"-s", sandbox,
		"-c", "approval_policy=\"never\"",
		"-c", fmt.Sprintf("sandbox_workspace_write.network_access=%t", opts.networkAccess),
	}
	if opts.model != "" {
		argv = append(argv, "--model", opts.model)
	}
	if opts.profile != "" {
		argv = append(argv, "--profile", opts.profile)
	}
	if opts.outputLast != "" {
		argv = append(argv, "--output-last-message", opts.outputLast)
	}
	return append(argv, "-")
}

func externalAdapterArgs(runner string, target string, opts *autoflowStepOptions, spec autoflow.PhaseSpec, sandbox string) []string {
	argv := []string{runner, "--target", target, "--phase", spec.Name, "--sandbox", sandbox}
	if opts.model != "" {
		argv = append(argv, "--model", opts.model)
	}
	if opts.profile != "" {
		argv = append(argv, "--profile", opts.profile)
	}
	if opts.networkAccess {
		argv = append(argv, "--network")
	}
	if opts.outputLast != "" {
		argv = append(argv, "--output", opts.outputLast)
	}
	if opts.printPrompt {
		argv = append(argv, "--print-prompt")
	}
	if opts.promptFile != "" {
		argv = append(argv, "--prompt-file", opts.promptFile)
	} else if opts.prompt != "" {
		argv = append(argv, "--prompt", opts.prompt)
	}
	return argv
}

func normalizeSandbox(sandbox string) (string, error) {
	if strings.TrimSpace(sandbox) == "" {
		return "workspace-write", nil
	}
	switch sandbox {
	case "read-only", "workspace-write", "danger-full-access":
		return sandbox, nil
	default:
		return "", fmt.Errorf("unsupported sandbox %q", sandbox)
	}
}

func readTaskPrompt(cmd *cobra.Command, opts *autoflowStepOptions, required bool) (string, error) {
	if opts.promptFile != "" {
		data, err := os.ReadFile(opts.promptFile)
		if err != nil {
			return "", fmt.Errorf("read prompt file: %w", err)
		}
		prompt := string(data)
		if required && strings.TrimSpace(prompt) == "" {
			return "", fmt.Errorf("prompt file is empty: %s", opts.promptFile)
		}
		return prompt, nil
	}
	if opts.prompt != "" {
		return opts.prompt, nil
	}
	if !required {
		return "", nil
	}

	in := cmd.InOrStdin()
	if file, ok := in.(*os.File); ok {
		info, err := file.Stat()
		if err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return "", fmt.Errorf("provide --prompt, --prompt-file, or piped stdin")
		}
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return "", fmt.Errorf("read prompt from stdin: %w", err)
	}
	prompt := string(data)
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("provide --prompt, --prompt-file, or piped stdin")
	}
	return prompt, nil
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

func printScaffoldResult(cmd *cobra.Command, result autoflow.ScaffoldResult, missingLabel string, existingLabel string) {
	for _, artifact := range result.Artifacts {
		label := missingLabel
		if artifact.Exists {
			label = existingLabel
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", label, artifact.RelativePath)
	}
}

func printGitignoreAdvice(cmd *cobra.Command, includeGitignore bool) {
	if includeGitignore {
		return
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "note: add %s to .gitignore or rerun with --gitignore to ignore Orca local state\n", autoflow.GitignoreAdvice())
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
			return !isShellSafeArgRune(r)
		}) == -1 {
			quoted = append(quoted, arg)
			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
	}
	return strings.Join(quoted, " ")
}

func isShellSafeArgRune(r rune) bool {
	switch {
	case r == '/' || r == '-' || r == '_' || r == '.' || r == ':' || r == '=':
		return true
	case r >= '0' && r <= '9':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= 'a' && r <= 'z':
		return true
	default:
		return false
	}
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
	autoflowCmd.AddCommand(newAutoflowInitCmd(), newAutoflowStepCmd())
	rootCmd.AddCommand(autoflowCmd)
}

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

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

	metadata := autoflow.NewPhaseRunMetadata(opts.issue, spec.Name, time.Now())
	metadata.Adapter = opts.adapter
	metadata.Model = opts.model
	metadata.Sandbox = sandbox
	metadata.NetworkAccess = opts.networkAccess

	if opts.runner != "" {
		err = runExternalAutoflowAdapter(ctx, cmd, opts, target, spec, sandbox, &metadata)
	} else {
		err = runBuiltInCodexAdapter(ctx, cmd, opts, target, spec, sandbox, &metadata)
	}
	if err != nil {
		finishPhaseRunMetadata(&metadata, err)
		if saveErr := autoflow.SavePhaseRunMetadata(target, metadata); saveErr != nil {
			return fmt.Errorf("%w; additionally failed to save phase run metadata: %v", err, saveErr)
		}
		printPhaseRunDiagnostics(cmd, metadata)
		return err
	}
	if opts.dryRun || opts.printPrompt {
		return nil
	}

	outputs := autoflow.RenderPaths(target, opts.issue, spec.Outputs)
	missingOutputs := missingPaths(outputs)
	if len(missingOutputs) > 0 {
		err := fmt.Errorf("adapter completed but required output artifact(s) are missing: %s", strings.Join(relativizeAll(target, missingOutputs), ", "))
		finishPhaseRunMetadata(&metadata, err)
		if saveErr := autoflow.SavePhaseRunMetadata(target, metadata); saveErr != nil {
			return fmt.Errorf("%w; additionally failed to save phase run metadata: %v", err, saveErr)
		}
		printPhaseRunDiagnostics(cmd, metadata)
		return err
	}

	finishPhaseRunMetadata(&metadata, nil)
	if err := autoflow.SavePhaseRunMetadata(target, metadata); err != nil {
		return err
	}

	state := autoflow.State{
		Issue:              opts.issue,
		Active:             true,
		Phase:              spec.Next,
		LastCompletedPhase: spec.Name,
		Adapter:            opts.adapter,
		Model:              opts.model,
		LastRunMetadata:    metadata.MetadataPath,
		Artifacts:          artifactMap(target, opts.issue, spec),
	}
	if err := autoflow.SaveState(target, state); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "completed phase %s for issue #%d; next phase: %s\n", spec.Name, opts.issue, spec.Next)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "state: %s\n", autoflow.StatePath(target, opts.issue))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "metadata: %s\n", filepath.Join(target, filepath.FromSlash(metadata.MetadataPath)))
	return nil
}

func runBuiltInCodexAdapter(ctx context.Context, cmd *cobra.Command, opts *autoflowStepOptions, target string, spec autoflow.PhaseSpec, sandbox string, metadata *autoflow.PhaseRunMetadata) error {
	roleContract, roleSource, err := autoflow.RoleContract(target, spec.AgentType)
	if err != nil {
		return err
	}
	metadata.RoleContractSource = roleSource

	argv := codexAdapterArgs(codexExecutable(opts), target, opts, sandbox)
	metadata.Command = metadataCommandBoundary(argv)
	if opts.outputLast != "" {
		metadata.LastMessagePath = autoflow.RelativizePath(target, opts.outputLast)
	}
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

	logs, err := openPhaseRunLogs(cmd, target, metadata)
	if err != nil {
		return err
	}

	if _, err := exec.LookPath(argv[0]); err != nil {
		_ = logs.close()
		return fmt.Errorf("codex executable %q is not on PATH: %w", argv[0], err)
	}
	execCmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	execCmd.Dir = target
	execCmd.Stdin = strings.NewReader(composedPrompt)
	execCmd.Stdout = logs.stdout
	execCmd.Stderr = logs.stderr
	runErr := execCmd.Run()
	closeErr := logs.close()
	if runErr != nil {
		return fmt.Errorf("run codex adapter: %w", runErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}

func runExternalAutoflowAdapter(ctx context.Context, cmd *cobra.Command, opts *autoflowStepOptions, target string, spec autoflow.PhaseSpec, sandbox string, metadata *autoflow.PhaseRunMetadata) error {
	if _, err := os.Stat(opts.runner); err != nil {
		return fmt.Errorf("codex adapter runner not found at %s: %w", opts.runner, err)
	}
	argv := externalAdapterArgs(opts.runner, target, opts, spec, sandbox)
	metadata.Runner = opts.runner
	metadata.Command = metadataCommandBoundary(argv)

	if opts.dryRun {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "phase: %s\nagent_type: %s\nrunner: %s\n", spec.Name, spec.AgentType, opts.runner)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "command: %s\n", shellJoin(argv))
		return nil
	}

	logs, err := openPhaseRunLogs(cmd, target, metadata)
	if err != nil {
		return err
	}

	execCmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	execCmd.Dir = target
	execCmd.Stdout = logs.stdout
	execCmd.Stderr = logs.stderr
	if opts.prompt == "" && opts.promptFile == "" {
		execCmd.Stdin = cmd.InOrStdin()
	}
	runErr := execCmd.Run()
	closeErr := logs.close()
	if runErr != nil {
		return fmt.Errorf("run %s adapter: %w", opts.adapter, runErr)
	}
	if closeErr != nil {
		return closeErr
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

func metadataCommandBoundary(argv []string) []string {
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--prompt" && i+1 < len(argv) {
			out = append(out, arg, "[REDACTED]")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--prompt=") {
			out = append(out, "--prompt=[REDACTED]")
			continue
		}
		out = append(out, arg)
	}
	return out
}

type phaseRunLogs struct {
	stdout    io.Writer
	stderr    io.Writer
	redactors []*redactingLogWriter
	files     []*os.File
}

func openPhaseRunLogs(cmd *cobra.Command, target string, metadata *autoflow.PhaseRunMetadata) (phaseRunLogs, error) {
	stdoutFile, err := openPhaseRunLogFile(target, metadata.Logs.Stdout)
	if err != nil {
		return phaseRunLogs{}, err
	}
	stderrFile, err := openPhaseRunLogFile(target, metadata.Logs.Stderr)
	if err != nil {
		_ = stdoutFile.Close()
		return phaseRunLogs{}, err
	}
	consoleLock := &sync.Mutex{}
	stdoutRedactor := &redactingLogWriter{w: stdoutFile}
	stderrRedactor := &redactingLogWriter{w: stderrFile}
	return phaseRunLogs{
		stdout:    io.MultiWriter(lockedWriter{w: cmd.OutOrStdout(), mu: consoleLock}, stdoutRedactor),
		stderr:    io.MultiWriter(lockedWriter{w: cmd.ErrOrStderr(), mu: consoleLock}, stderrRedactor),
		redactors: []*redactingLogWriter{stdoutRedactor, stderrRedactor},
		files:     []*os.File{stdoutFile, stderrFile},
	}, nil
}

func openPhaseRunLogFile(target string, relPath string) (*os.File, error) {
	path := autoflow.PhaseRunLogPath(target, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", relPath, err)
	}
	return file, nil
}

func (l phaseRunLogs) close() error {
	var firstErr error
	for _, redactor := range l.redactors {
		if err := redactor.Flush(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, file := range l.files {
		if err := file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type lockedWriter struct {
	w  io.Writer
	mu *sync.Mutex
}

func (w lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

type redactingLogWriter struct {
	w       io.Writer
	mu      sync.Mutex
	pending string
}

func (w *redactingLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pending += string(p)
	for {
		newline := strings.IndexByte(w.pending, '\n')
		if newline == -1 {
			break
		}
		line := w.pending[:newline+1]
		w.pending = w.pending[newline+1:]
		if _, err := w.w.Write([]byte(redactLogChunk(line))); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *redactingLogWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending == "" {
		return nil
	}
	_, err := w.w.Write([]byte(redactLogChunk(w.pending)))
	w.pending = ""
	return err
}

var secretLogPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)([A-Z0-9_]*(?:TOKEN|SECRET|PASSWORD|PASS|API_KEY|ACCESS_KEY|PRIVATE_KEY)[A-Z0-9_]*\s*=\s*)\S+`),
	regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[A-Za-z0-9._-]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{8,}`),
}

func redactLogChunk(s string) string {
	s = secretLogPatterns[0].ReplaceAllString(s, `${1}[REDACTED]`)
	s = secretLogPatterns[1].ReplaceAllString(s, `${1}[REDACTED]`)
	s = secretLogPatterns[2].ReplaceAllString(s, `[REDACTED]`)
	return s
}

func finishPhaseRunMetadata(metadata *autoflow.PhaseRunMetadata, runErr error) {
	finished := time.Now().UTC()
	metadata.FinishedAt = finished.Format(time.RFC3339Nano)
	if started, err := time.Parse(time.RFC3339Nano, metadata.StartedAt); err == nil {
		metadata.DurationMillis = finished.Sub(started).Milliseconds()
	}
	exitCode := 0
	metadata.ExitCode = &exitCode
	metadata.Status = "success"
	if runErr == nil {
		return
	}
	metadata.Status = "error"
	metadata.Error = runErr.Error()
	if code, ok := exitCodeFromError(runErr); ok {
		exitCode = code
		metadata.ExitCode = &exitCode
		return
	}
	metadata.ExitCode = nil
}

func exitCodeFromError(err error) (int, bool) {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true
	}
	return 0, false
}

func printPhaseRunDiagnostics(cmd *cobra.Command, metadata autoflow.PhaseRunMetadata) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "metadata: %s\n", metadata.MetadataPath)
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "stdout log: %s\n", metadata.Logs.Stdout)
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "stderr log: %s\n", metadata.Logs.Stderr)
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

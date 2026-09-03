package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoflowInitCreatesTemplatesThatCanPrintRedPrompt(t *testing.T) {
	target := t.TempDir()
	cmd := newAutoflowInitCmd()
	var opts autoflowInitOptions
	opts.target = target
	opts.issue = 123

	out, err := runAutoflowInitCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "create: .autoflow")
	assert.Contains(t, out, "create: .autoflow/issue-123-verification-design.md")
	assert.Contains(t, out, "create: .autoflow/issue-123-red-prompt.md")
	assert.Contains(t, out, "create: .autoflow/issue-123-green-prompt.md")
	assert.Contains(t, out, "create: .autoflow/issue-123-verify-arbitration-prompt.md")
	assert.Contains(t, out, "create: .autoflow/issue-123-refine-impl-prompt.md")
	assert.Contains(t, out, "create: .autoflow/issue-123-refine-test-reconfirm-prompt.md")
	assert.Contains(t, out, "create: .autoflow/issue-123-gate-quality-prompt.md")
	assert.Contains(t, out, ".autoflow/issue-*-orca.json")

	assert.FileExists(t, filepath.Join(target, ".autoflow", "issue-123-verification-design.md"))
	assert.FileExists(t, filepath.Join(target, ".autoflow", "issue-123-red-prompt.md"))
	assert.FileExists(t, filepath.Join(target, ".autoflow", "issue-123-green-prompt.md"))
	assert.FileExists(t, filepath.Join(target, ".autoflow", "issue-123-verify-arbitration-prompt.md"))
	assert.FileExists(t, filepath.Join(target, ".autoflow", "issue-123-refine-impl-prompt.md"))
	assert.FileExists(t, filepath.Join(target, ".autoflow", "issue-123-refine-test-reconfirm-prompt.md"))
	assert.FileExists(t, filepath.Join(target, ".autoflow", "issue-123-gate-quality-prompt.md"))

	stepCmd := newAutoflowStepCmd()
	var stepOpts autoflowStepOptions
	stepOpts.target = target
	stepOpts.issue = 123
	stepOpts.phase = "red"
	stepOpts.adapter = "codex"
	stepOpts.promptFile = filepath.Join(target, ".autoflow", "issue-123-red-prompt.md")
	stepOpts.printPrompt = true

	prompt, err := runAutoflowStepCapture(stepCmd, &stepOpts)
	require.NoError(t, err)
	assert.Contains(t, prompt, "AutoFlow phase: red")
	assert.Contains(t, prompt, "Required input artifacts:\n- .autoflow/issue-123-verification-design.md")
	assert.Contains(t, prompt, "Task prompt:\n# AutoFlow Red Prompt for Issue #123")
}

func TestAutoflowInitDryRunDoesNotCreateFiles(t *testing.T) {
	target := t.TempDir()
	cmd := newAutoflowInitCmd()
	var opts autoflowInitOptions
	opts.target = target
	opts.issue = 123
	opts.dryRun = true

	out, err := runAutoflowInitCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "would create: .autoflow")
	assert.Contains(t, out, "would create: .autoflow/issue-123-verification-design.md")
	assert.NoDirExists(t, filepath.Join(target, ".autoflow"))
}

func TestAutoflowInitPreservesExistingArtifacts(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	designPath := filepath.Join(target, ".autoflow", "issue-123-verification-design.md")
	require.NoError(t, os.WriteFile(designPath, []byte("custom design\n"), 0o644))

	cmd := newAutoflowInitCmd()
	var opts autoflowInitOptions
	opts.target = target
	opts.issue = 123

	out, err := runAutoflowInitCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "exists: .autoflow")
	assert.Contains(t, out, "exists: .autoflow/issue-123-verification-design.md")
	assert.Contains(t, out, "create: .autoflow/issue-123-red-prompt.md")

	data, err := os.ReadFile(designPath)
	require.NoError(t, err)
	assert.Equal(t, "custom design\n", string(data))
}

func TestAutoflowInitCanAddGitignoreEntry(t *testing.T) {
	target := t.TempDir()
	gitignorePath := filepath.Join(target, ".gitignore")
	require.NoError(t, os.WriteFile(gitignorePath, []byte("dist"), 0o644))

	cmd := newAutoflowInitCmd()
	var opts autoflowInitOptions
	opts.target = target
	opts.issue = 123
	opts.includeGitignore = true

	out, err := runAutoflowInitCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "create: .gitignore")

	data, err := os.ReadFile(gitignorePath)
	require.NoError(t, err)
	assert.Equal(t, "dist\n.autoflow/issue-*-orca.json\n.autoflow/logs/\n", string(data))
}

func TestAutoflowStep_DryRunRequiresInputs(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	cmd := newAutoflowStepCmd()

	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "red"
	opts.adapter = "codex"
	opts.dryRun = true

	err := runAutoflowStep(cmd, &opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".autoflow/issue-123-verification-design.md")
}

func TestAutoflowStep_DryRunRequiresPriorPhaseInputs(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-red.md"), []byte("red\n"), 0o644))
	cmd := newAutoflowStepCmd()

	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "verify-arbitration"
	opts.adapter = "codex"
	opts.dryRun = true

	err := runAutoflowStep(cmd, &opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required input artifact(s): .autoflow/issue-123-green.md")
	assert.NotContains(t, err.Error(), target)
}

func TestAutoflowStep_DryRunPrintsCommand(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "red"
	opts.adapter = "codex"
	opts.model = "gpt-5-codex"
	opts.prompt = "write tests"
	opts.dryRun = true

	out, err := runAutoflowStepCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "phase: red")
	assert.Contains(t, out, "agent_type: autoflow-tester")
	assert.Contains(t, out, "adapter: built-in codex")
	assert.Contains(t, out, "role_contract: built-in:autoflow-tester")
	assert.Contains(t, out, "codex exec")
	assert.Contains(t, out, "--model gpt-5-codex")
}

func TestAutoflowStepPrintPromptSupportsVerifyArbitration(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-red.md"), []byte("red\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-green.md"), []byte("green\n"), 0o644))

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "verify-arbitration"
	opts.adapter = "codex"
	opts.prompt = "verify implementation"
	opts.printPrompt = true

	out, err := runAutoflowStepCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "AutoFlow phase: verify-arbitration")
	assert.Contains(t, out, "AutoFlow role: autoflow-verifier")
	assert.Contains(t, out, "Required input artifacts:\n- .autoflow/issue-123-verification-design.md\n- .autoflow/issue-123-red.md\n- .autoflow/issue-123-green.md")
	assert.Contains(t, out, "Required output artifacts:\n- .autoflow/issue-123-verify-arbitration.md")
}

func TestAutoflowStepPrintPromptUsesOpenGitHubIssueWhenPromptIsOmitted(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	require.NoError(t, runCommand(target, "git", "init", "-q"))
	require.NoError(t, runCommand(target, "git", "remote", "add", "origin", "git@github.com:orca/example.git"))

	fakebin := t.TempDir()
	writeFakeGH(t, fakebin, `{"title":"Add issue intake","body":"Use the issue body as the default task prompt.\n\n## Acceptance Criteria\n- title is included\n- body is included","state":"OPEN","labels":[{"name":"enhancement"}]}`)

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "red"
	opts.adapter = "codex"
	opts.printPrompt = true

	out, err := runAutoflowStepCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "Task prompt:\n# GitHub Issue #123: Add issue intake")
	assert.Contains(t, out, "State: OPEN")
	assert.Contains(t, out, "Labels: enhancement")
	assert.Contains(t, out, "Use the issue body as the default task prompt.")
	assert.Contains(t, out, "body is included")
}

func TestAutoflowStepPrintPromptSupportsExplicitRepo(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))

	fakebin := t.TempDir()
	writeFakeGH(t, fakebin, `{"title":"Explicit repo","body":"loaded through --repo","state":"OPEN","labels":[]}`)

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.repo = "orca/example"
	opts.phase = "red"
	opts.adapter = "codex"
	opts.printPrompt = true

	out, err := runAutoflowStepCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "# GitHub Issue #123: Explicit repo")
	assert.Contains(t, out, "loaded through --repo")
}

func TestAutoflowStepRejectsClosedGitHubIssue(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	require.NoError(t, runCommand(target, "git", "init", "-q"))
	require.NoError(t, runCommand(target, "git", "remote", "add", "origin", "git@github.com:orca/example.git"))

	fakebin := t.TempDir()
	writeFakeGH(t, fakebin, `{"title":"Done","body":"already complete","state":"CLOSED","labels":[]}`)

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "red"
	opts.adapter = "codex"
	opts.dryRun = true

	err := runAutoflowStep(cmd, &opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issue #123 is CLOSED")
	assert.Contains(t, err.Error(), "--allow-closed-issue")
}

func TestAutoflowStepAllowClosedGitHubIssueForReplay(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	require.NoError(t, runCommand(target, "git", "init", "-q"))
	require.NoError(t, runCommand(target, "git", "remote", "add", "origin", "git@github.com:orca/example.git"))

	fakebin := t.TempDir()
	writeFakeGH(t, fakebin, `{"title":"Replay closed issue","body":"closed body","state":"CLOSED","labels":[]}`)

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "red"
	opts.adapter = "codex"
	opts.printPrompt = true
	opts.allowClosedIssue = true

	out, err := runAutoflowStepCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "# GitHub Issue #123: Replay closed issue")
	assert.Contains(t, out, "State: CLOSED")
	assert.Contains(t, out, "closed body")
}

func TestAutoflowStepAllowClosedIssueWithLocalPromptDoesNotRequireGH(t *testing.T) {
	tests := []struct {
		name      string
		configure func(t *testing.T, target string, opts *autoflowStepOptions)
		want      string
	}{
		{
			name: "inline prompt",
			configure: func(t *testing.T, target string, opts *autoflowStepOptions) {
				opts.prompt = "local replay prompt"
			},
			want: "Task prompt:\nlocal replay prompt",
		},
		{
			name: "prompt file",
			configure: func(t *testing.T, target string, opts *autoflowStepOptions) {
				promptFile := filepath.Join(target, ".autoflow", "issue-123-red-prompt.md")
				require.NoError(t, os.WriteFile(promptFile, []byte("local replay file prompt\n"), 0o644))
				opts.promptFile = promptFile
			},
			want: "Task prompt:\nlocal replay file prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
			require.NoError(t, runCommand(target, "git", "init", "-q"))
			require.NoError(t, runCommand(target, "git", "remote", "add", "origin", "git@github.com:orca/example.git"))

			fakebin := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(fakebin, "gh"), []byte("#!/bin/sh\nprintf 'offline\\n' >&2\nexit 1\n"), 0o755))
			t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

			cmd := newAutoflowStepCmd()
			var opts autoflowStepOptions
			opts.target = target
			opts.issue = 123
			opts.phase = "red"
			opts.adapter = "codex"
			opts.printPrompt = true
			opts.allowClosedIssue = true
			tt.configure(t, target, &opts)

			out, err := runAutoflowStepCapture(cmd, &opts)
			require.NoError(t, err)
			assert.Contains(t, out, tt.want)
		})
	}
}

func TestAutoflowStepGitHubIssueIntakeReportsGHFailures(t *testing.T) {
	tests := []struct {
		name       string
		ghScript   string
		wantErr    string
		wantDetail string
	}{
		{
			name:       "not found",
			ghScript:   "#!/bin/sh\nprintf 'GraphQL: Could not resolve to an Issue with the number of 123.\\n' >&2\nexit 1\n",
			wantErr:    "could not read GitHub issue #123 with gh",
			wantDetail: "Could not resolve",
		},
		{
			name:       "offline",
			ghScript:   "#!/bin/sh\nprintf 'Post \"https://api.github.com/graphql\": dial tcp: lookup api.github.com: no such host\\n' >&2\nexit 1\n",
			wantErr:    "could not read GitHub issue #123 with gh",
			wantDetail: "no such host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
			require.NoError(t, runCommand(target, "git", "init", "-q"))
			require.NoError(t, runCommand(target, "git", "remote", "add", "origin", "git@github.com:orca/example.git"))

			fakebin := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(fakebin, "gh"), []byte(tt.ghScript), 0o755))
			t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

			cmd := newAutoflowStepCmd()
			var opts autoflowStepOptions
			opts.target = target
			opts.issue = 123
			opts.phase = "red"
			opts.adapter = "codex"
			opts.printPrompt = true

			_, err := runAutoflowStepCapture(cmd, &opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Contains(t, err.Error(), tt.wantDetail)
		})
	}
}

func TestAutoflowStepRunsBuiltInCodexAdapterAndWritesState(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	fakebin := t.TempDir()
	fakeCodex := filepath.Join(fakebin, "codex")
	require.NoError(t, os.WriteFile(fakeCodex, []byte("#!/bin/sh\nset -eu\nprintf '%s\\n' \"$@\" > .autoflow/codex.args\ncat > .autoflow/codex.prompt\nprintf 'stdout SECRET_TOKEN=topsecret\\n'\nprintf 'stderr authorization: Bearer abc123456789\\n' >&2\nprintf 'red\\n' > .autoflow/issue-123-red.md\n"), 0o755))

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "red"
	opts.adapter = "codex"
	opts.codexBin = fakeCodex
	opts.model = "gpt-5-codex"
	opts.prompt = "write tests"

	out, err := runAutoflowStepCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "completed phase red")
	assert.Contains(t, out, "metadata: ")

	args, err := os.ReadFile(filepath.Join(target, ".autoflow", "codex.args"))
	require.NoError(t, err)
	assert.Contains(t, string(args), "exec\n")
	assert.Contains(t, string(args), "--model\n")
	assert.Contains(t, string(args), "gpt-5-codex\n")

	prompt, err := os.ReadFile(filepath.Join(target, ".autoflow", "codex.prompt"))
	require.NoError(t, err)
	assert.Contains(t, string(prompt), "AutoFlow phase: red")
	assert.Contains(t, string(prompt), "AutoFlow role: autoflow-tester")
	assert.Contains(t, string(prompt), "Required output artifacts:\n- .autoflow/issue-123-red.md")
	assert.Contains(t, string(prompt), "Task prompt:\nwrite tests")

	stateBytes, err := os.ReadFile(filepath.Join(target, ".autoflow", "issue-123-orca.json"))
	require.NoError(t, err)
	var state map[string]any
	require.NoError(t, json.Unmarshal(stateBytes, &state))
	assert.Equal(t, float64(123), state["issue"])
	assert.Equal(t, true, state["active"])
	assert.Equal(t, "green", state["phase"])
	assert.Equal(t, "red", state["last_completed_phase"])
	assert.Equal(t, "codex", state["adapter"])
	metadataRel, ok := state["last_run_metadata"].(string)
	require.True(t, ok)
	assert.Contains(t, metadataRel, ".autoflow/logs/issue-123/red-")
	assert.Contains(t, metadataRel, "metadata.json")

	metadataBytes, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(metadataRel)))
	require.NoError(t, err)
	var metadata map[string]any
	require.NoError(t, json.Unmarshal(metadataBytes, &metadata))
	assert.Equal(t, "red", metadata["phase"])
	assert.Equal(t, "codex", metadata["adapter"])
	assert.Equal(t, "gpt-5-codex", metadata["model"])
	assert.Equal(t, "workspace-write", metadata["sandbox"])
	assert.Equal(t, false, metadata["network_access"])
	assert.Equal(t, "built-in:autoflow-tester", metadata["role_contract_source"])
	assert.Equal(t, "success", metadata["status"])
	assert.Equal(t, float64(0), metadata["exit_code"])
	command, ok := metadata["command"].([]any)
	require.True(t, ok)
	assert.Contains(t, command, fakeCodex)

	logs, ok := metadata["logs"].(map[string]any)
	require.True(t, ok)
	stdoutLog, ok := logs["stdout"].(string)
	require.True(t, ok)
	stderrLog, ok := logs["stderr"].(string)
	require.True(t, ok)
	stdoutBytes, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(stdoutLog)))
	require.NoError(t, err)
	assert.Contains(t, string(stdoutBytes), "SECRET_TOKEN=[REDACTED]")
	assert.NotContains(t, string(stdoutBytes), "topsecret")
	stderrBytes, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(stderrLog)))
	require.NoError(t, err)
	assert.Contains(t, string(stderrBytes), "authorization: Bearer [REDACTED]")
	assert.NotContains(t, string(stderrBytes), "abc123456789")
}

func TestAutoflowStepFailedBuiltInCodexWritesMetadataAndLogs(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	fakebin := t.TempDir()
	fakeCodex := filepath.Join(fakebin, "codex")
	require.NoError(t, os.WriteFile(fakeCodex, []byte("#!/bin/sh\nset -eu\ncat > .autoflow/codex.prompt\nprintf 'stdout API_KEY=secret-value\\n'\nprintf 'failed hard\\n' >&2\nexit 17\n"), 0o755))

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "red"
	opts.adapter = "codex"
	opts.codexBin = fakeCodex
	opts.prompt = "write tests"

	out, err := runAutoflowStepCapture(cmd, &opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run codex adapter")
	assert.Contains(t, out, "metadata: .autoflow/logs/issue-123/red-")
	assert.Contains(t, out, "stdout log: .autoflow/logs/issue-123/red-")
	assert.Contains(t, out, "stderr log: .autoflow/logs/issue-123/red-")
	assert.NoFileExists(t, filepath.Join(target, ".autoflow", "issue-123-orca.json"))

	matches, err := filepath.Glob(filepath.Join(target, ".autoflow", "logs", "issue-123", "red-*", "metadata.json"))
	require.NoError(t, err)
	require.Len(t, matches, 1)

	metadataBytes, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	var metadata map[string]any
	require.NoError(t, json.Unmarshal(metadataBytes, &metadata))
	assert.Equal(t, "error", metadata["status"])
	assert.Equal(t, float64(17), metadata["exit_code"])
	assert.Contains(t, metadata["error"], "run codex adapter")
	assert.Equal(t, "built-in:autoflow-tester", metadata["role_contract_source"])

	logs, ok := metadata["logs"].(map[string]any)
	require.True(t, ok)
	stdoutLog, ok := logs["stdout"].(string)
	require.True(t, ok)
	stdoutBytes, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(stdoutLog)))
	require.NoError(t, err)
	assert.Contains(t, string(stdoutBytes), "API_KEY=[REDACTED]")
	assert.NotContains(t, string(stdoutBytes), "secret-value")
	stderrLog, ok := logs["stderr"].(string)
	require.True(t, ok)
	stderrBytes, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(stderrLog)))
	require.NoError(t, err)
	assert.Contains(t, string(stderrBytes), "failed hard")
}

func TestMetadataCommandBoundaryRedactsInlinePrompt(t *testing.T) {
	got := metadataCommandBoundary([]string{
		"runner",
		"--prompt",
		"API_KEY=secret-value",
		"--model",
		"gpt-5-codex",
		"--prompt=SECRET_TOKEN=topsecret",
	})

	assert.Equal(t, []string{
		"runner",
		"--prompt",
		"[REDACTED]",
		"--model",
		"gpt-5-codex",
		"--prompt=[REDACTED]",
	}, got)
}

func TestRedactingLogWriterRedactsSecretsAcrossChunkBoundaries(t *testing.T) {
	var buf bytes.Buffer
	writer := &redactingLogWriter{w: &buf}

	n, err := writer.Write([]byte("SECRET_TOKEN="))
	require.NoError(t, err)
	assert.Equal(t, len("SECRET_TOKEN="), n)
	n, err = writer.Write([]byte("topsecret\nAuthorization: Bearer "))
	require.NoError(t, err)
	assert.Equal(t, len("topsecret\nAuthorization: Bearer "), n)
	n, err = writer.Write([]byte("abc123456789\n"))
	require.NoError(t, err)
	assert.Equal(t, len("abc123456789\n"), n)
	require.NoError(t, writer.Flush())

	assert.Contains(t, buf.String(), "SECRET_TOKEN=[REDACTED]")
	assert.Contains(t, buf.String(), "Authorization: Bearer [REDACTED]")
	assert.NotContains(t, buf.String(), "topsecret")
	assert.NotContains(t, buf.String(), "abc123456789")
}

func TestAutoflowStepRequiresPhaseOutputFromNewPhase(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-red.md"), []byte("red\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-green.md"), []byte("green\n"), 0o644))
	fakebin := t.TempDir()
	fakeCodex := filepath.Join(fakebin, "codex")
	require.NoError(t, os.WriteFile(fakeCodex, []byte("#!/bin/sh\nset -eu\ncat > .autoflow/codex.prompt\n"), 0o755))

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "verify-arbitration"
	opts.adapter = "codex"
	opts.codexBin = fakeCodex
	opts.prompt = "verify implementation"

	_, err := runAutoflowStepCapture(cmd, &opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adapter completed but required output artifact(s) are missing: .autoflow/issue-123-verify-arbitration.md")
	assert.NotContains(t, err.Error(), target)
}

func TestAutoflowStepExternalRunnerStillSupported(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(target, "scripts", "orca"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	runner := filepath.Join(target, "scripts", "orca", "codex-agent.sh")
	require.NoError(t, os.WriteFile(runner, []byte("#!/bin/sh\nset -eu\necho \"$@\" > .autoflow/runner.args\nprintf 'red\\n' > .autoflow/issue-123-red.md\n"), 0o755))

	cmd := newAutoflowStepCmd()
	var opts autoflowStepOptions
	opts.target = target
	opts.issue = 123
	opts.phase = "red"
	opts.adapter = "codex"
	opts.runner = runner
	opts.model = "gpt-5-codex"
	opts.prompt = "write tests"

	out, err := runAutoflowStepCapture(cmd, &opts)
	require.NoError(t, err)
	assert.Contains(t, out, "completed phase red")

	args, err := os.ReadFile(filepath.Join(target, ".autoflow", "runner.args"))
	require.NoError(t, err)
	assert.Contains(t, string(args), "--phase red")
	assert.Contains(t, string(args), "--model gpt-5-codex")
}

func runAutoflowStepCapture(cmd *cobra.Command, opts *autoflowStepOptions) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := runAutoflowStep(cmd, opts)
	return buf.String(), err
}

func runAutoflowInitCapture(cmd *cobra.Command, opts *autoflowInitOptions) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := runAutoflowInit(cmd, opts)
	return buf.String(), err
}

func runCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

func writeFakeGH(t *testing.T, fakebin string, issueJSON string) {
	t.Helper()
	fakeGH := filepath.Join(fakebin, "gh")
	script := "#!/bin/sh\nset -eu\ncat <<'JSON'\n" + issueJSON + "\nJSON\n"
	require.NoError(t, os.WriteFile(fakeGH, []byte(script), 0o755))
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

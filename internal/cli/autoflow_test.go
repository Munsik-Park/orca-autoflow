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

func TestAutoflowStepRejectsClosedGitHubIssue(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	require.NoError(t, runCommand(target, "git", "init", "-q"))
	require.NoError(t, runCommand(target, "git", "remote", "add", "origin", "git@github.com:orca/example.git"))

	fakebin := t.TempDir()
	fakeGH := filepath.Join(fakebin, "gh")
	require.NoError(t, os.WriteFile(fakeGH, []byte("#!/bin/sh\nprintf 'CLOSED\\n'\n"), 0o755))
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

func TestAutoflowStepRunsBuiltInCodexAdapterAndWritesState(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".autoflow"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, ".autoflow", "issue-123-verification-design.md"), []byte("design\n"), 0o644))
	fakebin := t.TempDir()
	fakeCodex := filepath.Join(fakebin, "codex")
	require.NoError(t, os.WriteFile(fakeCodex, []byte("#!/bin/sh\nset -eu\nprintf '%s\\n' \"$@\" > .autoflow/codex.args\ncat > .autoflow/codex.prompt\nprintf 'red\\n' > .autoflow/issue-123-red.md\n"), 0o755))

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

func runCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

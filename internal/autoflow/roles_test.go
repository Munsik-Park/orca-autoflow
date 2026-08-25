package autoflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleContractFallsBackToBuiltInContract(t *testing.T) {
	body, source, err := RoleContract(t.TempDir(), "autoflow-tester")
	require.NoError(t, err)

	assert.Equal(t, "built-in:autoflow-tester", source)
	assert.Contains(t, body, "AutoFlow Test AI")
}

func TestRoleContractPrefersTargetLocalAgentFile(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".claude", "agents"), 0o755))
	rolePath := filepath.Join(target, ".claude", "agents", "autoflow-tester.md")
	require.NoError(t, os.WriteFile(rolePath, []byte("---\nname: autoflow-tester\n---\n\nlocal contract\n"), 0o644))

	body, source, err := RoleContract(target, "autoflow-tester")
	require.NoError(t, err)

	assert.Equal(t, rolePath, source)
	assert.Equal(t, "local contract\n", body)
}

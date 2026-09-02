package autoflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleContractFallsBackToBuiltInContract(t *testing.T) {
	tests := []struct {
		agentType string
		contains  string
	}{
		{agentType: "autoflow-tester", contains: "AutoFlow Test AI"},
		{agentType: "autoflow-implementer", contains: "AutoFlow Developer AI"},
		{agentType: "autoflow-verifier", contains: "AutoFlow Verification AI"},
		{agentType: "autoflow-refiner", contains: "AutoFlow Refinement AI"},
		{agentType: "autoflow-test-reconfirmer", contains: "AutoFlow Test Reconfirmation AI"},
		{agentType: "autoflow-quality-gate", contains: "AutoFlow Quality Gate AI"},
	}

	for _, tt := range tests {
		t.Run(tt.agentType, func(t *testing.T) {
			body, source, err := RoleContract(t.TempDir(), tt.agentType)
			require.NoError(t, err)

			assert.Equal(t, "built-in:"+tt.agentType, source)
			assert.Contains(t, body, tt.contains)
		})
	}
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

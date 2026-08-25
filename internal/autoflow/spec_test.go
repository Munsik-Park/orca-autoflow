package autoflow

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupPhase_Red(t *testing.T) {
	spec, err := LookupPhase("red")
	require.NoError(t, err)

	assert.Equal(t, "autoflow-tester", spec.AgentType)
	assert.Equal(t, []string{".autoflow/issue-{issue}-verification-design.md"}, spec.Inputs)
	assert.Equal(t, []string{".autoflow/issue-{issue}-red.md"}, spec.Outputs)
	assert.Equal(t, "green", spec.Next)
}

func TestLookupPhase_Unsupported(t *testing.T) {
	_, err := LookupPhase("diagnose")
	require.Error(t, err)
}

func TestRenderPaths(t *testing.T) {
	got := RenderPaths("/repo", 123, []string{".autoflow/issue-{issue}-red.md"})
	assert.Equal(t, []string{filepath.Join("/repo", ".autoflow", "issue-123-red.md")}, got)
}

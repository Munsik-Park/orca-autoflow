package autoflow

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupPhase_SupportedPhases(t *testing.T) {
	tests := []struct {
		name      string
		agentType string
		inputs    []string
		outputs   []string
		next      string
	}{
		{
			name:      "red",
			agentType: "autoflow-tester",
			inputs:    []string{".autoflow/issue-{issue}-verification-design.md"},
			outputs:   []string{".autoflow/issue-{issue}-red.md"},
			next:      "green",
		},
		{
			name:      "green",
			agentType: "autoflow-implementer",
			inputs: []string{
				".autoflow/issue-{issue}-verification-design.md",
				".autoflow/issue-{issue}-red.md",
			},
			outputs: []string{".autoflow/issue-{issue}-green.md"},
			next:    "verify-arbitration",
		},
		{
			name:      "verify-arbitration",
			agentType: "autoflow-verifier",
			inputs: []string{
				".autoflow/issue-{issue}-verification-design.md",
				".autoflow/issue-{issue}-red.md",
				".autoflow/issue-{issue}-green.md",
			},
			outputs: []string{".autoflow/issue-{issue}-verify-arbitration.md"},
			next:    "refine-impl",
		},
		{
			name:      "refine-impl",
			agentType: "autoflow-refiner",
			inputs: []string{
				".autoflow/issue-{issue}-verification-design.md",
				".autoflow/issue-{issue}-red.md",
				".autoflow/issue-{issue}-green.md",
				".autoflow/issue-{issue}-verify-arbitration.md",
			},
			outputs: []string{".autoflow/issue-{issue}-refine-impl.md"},
			next:    "refine-test-reconfirm",
		},
		{
			name:      "refine-test-reconfirm",
			agentType: "autoflow-test-reconfirmer",
			inputs: []string{
				".autoflow/issue-{issue}-verification-design.md",
				".autoflow/issue-{issue}-red.md",
				".autoflow/issue-{issue}-green.md",
				".autoflow/issue-{issue}-verify-arbitration.md",
				".autoflow/issue-{issue}-refine-impl.md",
			},
			outputs: []string{".autoflow/issue-{issue}-refine-test-reconfirm.md"},
			next:    "gate-quality",
		},
		{
			name:      "gate-quality",
			agentType: "autoflow-quality-gate",
			inputs: []string{
				".autoflow/issue-{issue}-verification-design.md",
				".autoflow/issue-{issue}-red.md",
				".autoflow/issue-{issue}-green.md",
				".autoflow/issue-{issue}-verify-arbitration.md",
				".autoflow/issue-{issue}-refine-impl.md",
				".autoflow/issue-{issue}-refine-test-reconfirm.md",
			},
			outputs: []string{".autoflow/issue-{issue}-gate-quality.md"},
			next:    "complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := LookupPhase(tt.name)
			require.NoError(t, err)

			assert.Equal(t, tt.name, spec.Name)
			assert.Equal(t, tt.agentType, spec.AgentType)
			assert.Equal(t, tt.inputs, spec.Inputs)
			assert.Equal(t, tt.outputs, spec.Outputs)
			assert.Equal(t, tt.next, spec.Next)
		})
	}
}

func TestSupportedPhasesReturnsExecutionOrder(t *testing.T) {
	assert.Equal(t, []string{
		"red",
		"green",
		"verify-arbitration",
		"refine-impl",
		"refine-test-reconfirm",
		"gate-quality",
	}, SupportedPhases())
}

func TestLookupPhase_Unsupported(t *testing.T) {
	_, err := LookupPhase("diagnose")
	require.Error(t, err)
}

func TestRenderPaths(t *testing.T) {
	got := RenderPaths("/repo", 123, []string{".autoflow/issue-{issue}-red.md"})
	assert.Equal(t, []string{filepath.Join("/repo", ".autoflow", "issue-123-red.md")}, got)
}

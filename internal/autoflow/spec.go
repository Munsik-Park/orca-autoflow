package autoflow

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PhaseSpec defines one artifact-based AutoFlow phase boundary.
type PhaseSpec struct {
	Name       string
	AgentType  string
	Inputs     []string
	Outputs    []string
	Next       string
	ModelHint  string
	EffortHint string
}

var phases = map[string]PhaseSpec{
	"red": {
		Name:      "red",
		AgentType: "autoflow-tester",
		Inputs: []string{
			".autoflow/issue-{issue}-verification-design.md",
		},
		Outputs: []string{
			".autoflow/issue-{issue}-red.md",
		},
		Next:       "green",
		ModelHint:  "sonnet",
		EffortHint: "inherit",
	},
	"green": {
		Name:      "green",
		AgentType: "autoflow-implementer",
		Inputs: []string{
			".autoflow/issue-{issue}-verification-design.md",
			".autoflow/issue-{issue}-red.md",
		},
		Outputs: []string{
			".autoflow/issue-{issue}-green.md",
		},
		Next:       "verify",
		ModelHint:  "opus",
		EffortHint: "inherit",
	},
}

// LookupPhase returns the spec for a supported phase.
func LookupPhase(name string) (PhaseSpec, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	spec, ok := phases[key]
	if !ok {
		return PhaseSpec{}, fmt.Errorf("unsupported autoflow phase %q", name)
	}
	return spec, nil
}

// SupportedPhases returns the current phase names in execution order.
func SupportedPhases() []string {
	return []string{"red", "green"}
}

// RenderPaths expands the issue placeholder and joins each path to targetRoot.
func RenderPaths(targetRoot string, issue int, patterns []string) []string {
	out := make([]string, 0, len(patterns))
	token := fmt.Sprintf("%d", issue)
	for _, pattern := range patterns {
		rel := strings.ReplaceAll(pattern, "{issue}", token)
		out = append(out, filepath.Join(targetRoot, filepath.FromSlash(rel)))
	}
	return out
}

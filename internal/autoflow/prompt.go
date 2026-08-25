package autoflow

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PromptRequest contains the runtime-neutral AutoFlow context passed to an
// adapter-specific LLM process.
type PromptRequest struct {
	Issue        int
	TargetRoot   string
	Phase        PhaseSpec
	RoleContract string
	RoleSource   string
	TaskPrompt   string
}

// ComposePrompt renders the prompt sent to Codex for a single AutoFlow phase.
func ComposePrompt(req PromptRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are running AutoFlow under Orca through Codex.\n")
	fmt.Fprintf(&b, "Use Orca's artifact boundaries as the execution protocol. Do not rely on Claude-specific plugin, hook, workflow, or subagent features.\n\n")
	fmt.Fprintf(&b, "Issue: #%d\n", req.Issue)
	fmt.Fprintf(&b, "AutoFlow phase: %s\n", req.Phase.Name)
	fmt.Fprintf(&b, "AutoFlow role: %s\n", req.Phase.AgentType)
	fmt.Fprintf(&b, "Policy model hint: %s\n", fallback(req.Phase.ModelHint, "unmapped"))
	fmt.Fprintf(&b, "Policy effort hint: %s\n", fallback(req.Phase.EffortHint, "unmapped"))
	fmt.Fprintf(&b, "Target root: %s\n", req.TargetRoot)
	fmt.Fprintf(&b, "Role contract source: %s\n\n", req.RoleSource)

	fmt.Fprintf(&b, "Required input artifacts:\n")
	for _, path := range RenderPaths(req.TargetRoot, req.Issue, req.Phase.Inputs) {
		fmt.Fprintf(&b, "- %s\n", relPath(req.TargetRoot, path))
	}
	fmt.Fprintf(&b, "\nRequired output artifacts:\n")
	for _, path := range RenderPaths(req.TargetRoot, req.Issue, req.Phase.Outputs) {
		fmt.Fprintf(&b, "- %s\n", relPath(req.TargetRoot, path))
	}

	fmt.Fprintf(&b, "\nRole contract:\n%s\n", strings.TrimSpace(req.RoleContract))
	fmt.Fprintf(&b, "\nTask prompt:\n%s\n", strings.TrimSpace(req.TaskPrompt))
	return b.String()
}

func fallback(value string, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}

func relPath(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

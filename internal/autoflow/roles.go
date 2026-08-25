package autoflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var builtInRoleContracts = map[string]string{
	"autoflow-tester": strings.TrimSpace(`
You are the AutoFlow Test AI for an Orca-run phase.

Rules:
- Use the verification design and issue acceptance criteria as the source of truth.
- Change test files only. Treat implementation code as read-only.
- Write only tests that are necessary to prove the requested behavior.
- For RED, confirm that newly added driving or regression tests fail for the expected reason.
- For a later re-run, confirm that the same tests pass without changing implementation code.
- Run the narrowest useful test command and include its result in the phase artifact.
- Do not edit Orca state files such as .autoflow/issue-*-orca.json.
- Write the required output artifact before finishing.

Report in the required phase artifact:
- What behavior is protected by the tests.
- Which files changed.
- The exact test command and result.
- Any gap that could not be automated.
`) + "\n",
	"autoflow-implementer": strings.TrimSpace(`
You are the AutoFlow Developer AI for an Orca-run phase.

Rules:
- Use the verification design, RED artifact, and issue acceptance criteria as the source of truth.
- Make the smallest implementation change that satisfies the agreed scope.
- Do not change tests during GREEN unless the prompt explicitly assigns test maintenance.
- Do not introduce speculative behavior, broad refactors, or unrelated cleanup.
- Run the narrowest useful verification command and include its result in the phase artifact.
- Do not edit Orca state files such as .autoflow/issue-*-orca.json.
- Write the required output artifact before finishing.

Report in the required phase artifact:
- What changed and why it is in scope.
- Which files changed.
- The exact verification command and result.
- Any remaining risk or manual follow-up.
`) + "\n",
}

// RoleContract returns a target-local role contract when present, otherwise a
// built-in Orca contract for the supported AutoFlow role.
func RoleContract(targetRoot string, agentType string) (body string, source string, err error) {
	for _, candidate := range []string{
		filepath.Join(targetRoot, ".claude", "agents", agentType+".md"),
		filepath.Join(targetRoot, ".claude", "autoflow", "agents", agentType+".md"),
	} {
		data, readErr := os.ReadFile(candidate)
		if readErr == nil {
			return stripFrontMatter(string(data)), candidate, nil
		}
		if !os.IsNotExist(readErr) {
			return "", "", fmt.Errorf("read role contract %s: %w", candidate, readErr)
		}
	}

	body, ok := builtInRoleContracts[agentType]
	if !ok {
		return "", "", fmt.Errorf("unsupported autoflow role %q", agentType)
	}
	return body, "built-in:" + agentType, nil
}

func stripFrontMatter(body string) string {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return body
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.TrimLeft(strings.Join(lines[i+1:], "\n"), "\n")
		}
	}
	return body
}

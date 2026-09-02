package autoflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const localStateIgnorePattern = ".autoflow/issue-*-orca.json"

// ScaffoldOptions controls creation of target-repository AutoFlow artifacts.
type ScaffoldOptions struct {
	TargetRoot       string
	Issue            int
	IncludeGitignore bool
}

// ScaffoldArtifact describes one filesystem path managed by the scaffold step.
type ScaffoldArtifact struct {
	Path         string
	RelativePath string
	Kind         string
	Exists       bool
}

// ScaffoldResult reports the artifacts that were created or preserved.
type ScaffoldResult struct {
	Artifacts []ScaffoldArtifact
}

// PlanScaffold returns the artifacts needed to run AutoFlow for an issue.
func PlanScaffold(opts ScaffoldOptions) (ScaffoldResult, error) {
	if opts.Issue <= 0 {
		return ScaffoldResult{}, fmt.Errorf("issue must be a positive integer")
	}
	target, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return ScaffoldResult{}, fmt.Errorf("resolve target: %w", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return ScaffoldResult{}, fmt.Errorf("inspect target: %w", err)
	}
	if !info.IsDir() {
		return ScaffoldResult{}, fmt.Errorf("target is not a directory: %s", target)
	}

	relPaths := []string{
		".autoflow",
		fmt.Sprintf(".autoflow/issue-%d-verification-design.md", opts.Issue),
		fmt.Sprintf(".autoflow/issue-%d-red-prompt.md", opts.Issue),
		fmt.Sprintf(".autoflow/issue-%d-green-prompt.md", opts.Issue),
	}
	if opts.IncludeGitignore {
		relPaths = append(relPaths, ".gitignore")
	}

	artifacts := make([]ScaffoldArtifact, 0, len(relPaths))
	for _, rel := range relPaths {
		path := filepath.Join(target, filepath.FromSlash(rel))
		_, statErr := os.Stat(path)
		exists := statErr == nil
		if statErr != nil && !os.IsNotExist(statErr) {
			return ScaffoldResult{}, fmt.Errorf("inspect %s: %w", rel, statErr)
		}
		kind := "file"
		switch rel {
		case ".autoflow":
			kind = "directory"
		case ".gitignore":
			kind = "gitignore-entry"
			if exists {
				data, err := os.ReadFile(path)
				if err != nil {
					return ScaffoldResult{}, fmt.Errorf("read .gitignore: %w", err)
				}
				exists = GitignoreContainsLocalState(string(data))
			}
		}
		artifacts = append(artifacts, ScaffoldArtifact{
			Path:         path,
			RelativePath: rel,
			Kind:         kind,
			Exists:       exists,
		})
	}
	return ScaffoldResult{Artifacts: artifacts}, nil
}

// CreateScaffold writes missing AutoFlow templates and preserves existing files.
func CreateScaffold(opts ScaffoldOptions) (ScaffoldResult, error) {
	result, err := PlanScaffold(opts)
	if err != nil {
		return ScaffoldResult{}, err
	}

	for _, artifact := range result.Artifacts {
		if artifact.Exists {
			continue
		}
		if artifact.Kind == "directory" {
			if err := os.MkdirAll(artifact.Path, 0o755); err != nil {
				return ScaffoldResult{}, fmt.Errorf("create %s: %w", artifact.RelativePath, err)
			}
			continue
		}
		if artifact.Kind == "gitignore-entry" {
			if err := addGitignoreEntry(artifact.Path); err != nil {
				return ScaffoldResult{}, fmt.Errorf("update .gitignore: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(artifact.Path), 0o755); err != nil {
			return ScaffoldResult{}, fmt.Errorf("create parent for %s: %w", artifact.RelativePath, err)
		}
		content, err := scaffoldContent(opts.Issue, artifact.RelativePath)
		if err != nil {
			return ScaffoldResult{}, err
		}
		if err := writeNewFile(artifact.Path, content); err != nil {
			return ScaffoldResult{}, fmt.Errorf("create %s: %w", artifact.RelativePath, err)
		}
	}
	return result, nil
}

func addGitignoreEntry(path string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	body := string(data)
	if GitignoreContainsLocalState(body) {
		return nil
	}
	prefix := ""
	if strings.TrimSpace(body) != "" && !strings.HasSuffix(body, "\n") {
		prefix = "\n"
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(prefix + localStateIgnorePattern + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func scaffoldContent(issue int, relPath string) (string, error) {
	switch relPath {
	case fmt.Sprintf(".autoflow/issue-%d-verification-design.md", issue):
		return verificationDesignTemplate(issue), nil
	case fmt.Sprintf(".autoflow/issue-%d-red-prompt.md", issue):
		return redPromptTemplate(issue), nil
	case fmt.Sprintf(".autoflow/issue-%d-green-prompt.md", issue):
		return greenPromptTemplate(issue), nil
	case ".gitignore":
		return localStateIgnorePattern + "\n", nil
	default:
		return "", fmt.Errorf("no scaffold template for %s", relPath)
	}
}

func writeNewFile(path string, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func verificationDesignTemplate(issue int) string {
	return fmt.Sprintf(`# AutoFlow Verification Design for Issue #%d

## Issue
Paste the issue title and body here.

## Expected Behavior
- 

## Test Strategy
- 

## Acceptance Criteria
- 

## Notes
- 
`, issue)
}

func redPromptTemplate(issue int) string {
	return fmt.Sprintf(`# AutoFlow Red Prompt for Issue #%d

Read .autoflow/issue-%d-verification-design.md, inspect the target repository, and write failing tests that capture the required behavior.

Write the phase report to .autoflow/issue-%d-red.md.
`, issue, issue, issue)
}

func greenPromptTemplate(issue int) string {
	return fmt.Sprintf(`# AutoFlow Green Prompt for Issue #%d

Read .autoflow/issue-%d-verification-design.md and .autoflow/issue-%d-red.md, then implement the smallest production change that makes the failing tests pass.

Write the phase report to .autoflow/issue-%d-green.md.
`, issue, issue, issue, issue)
}

// GitignoreAdvice returns the ignore pattern for Orca-owned local state.
func GitignoreAdvice() string {
	return localStateIgnorePattern
}

// GitignoreContainsLocalState reports whether a .gitignore body already ignores
// Orca's per-issue local state files.
func GitignoreContainsLocalState(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == localStateIgnorePattern {
			return true
		}
	}
	return false
}

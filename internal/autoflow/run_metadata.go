package autoflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PhaseRunMetadata records one AutoFlow adapter execution boundary.
type PhaseRunMetadata struct {
	Issue              int          `json:"issue"`
	Phase              string       `json:"phase"`
	Adapter            string       `json:"adapter"`
	Model              string       `json:"model,omitempty"`
	Sandbox            string       `json:"sandbox"`
	NetworkAccess      bool         `json:"network_access"`
	RoleContractSource string       `json:"role_contract_source,omitempty"`
	Runner             string       `json:"runner,omitempty"`
	Command            []string     `json:"command"`
	StartedAt          string       `json:"started_at"`
	FinishedAt         string       `json:"finished_at,omitempty"`
	DurationMillis     int64        `json:"duration_millis,omitempty"`
	Status             string       `json:"status"`
	ExitCode           *int         `json:"exit_code,omitempty"`
	Error              string       `json:"error,omitempty"`
	Logs               PhaseRunLogs `json:"logs"`
	LastMessagePath    string       `json:"last_message_path,omitempty"`
	MetadataPath       string       `json:"metadata_path"`
}

// PhaseRunLogs points to log files for a phase execution.
type PhaseRunLogs struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// NewPhaseRunMetadata creates metadata paths for a phase execution.
func NewPhaseRunMetadata(issue int, phase string, started time.Time) PhaseRunMetadata {
	stamp := started.UTC().Format("20060102T150405.000000000Z")
	baseRel := filepath.ToSlash(filepath.Join(".autoflow", "logs", fmt.Sprintf("issue-%d", issue), phase+"-"+stamp))
	metadataRel := filepath.ToSlash(filepath.Join(baseRel, "metadata.json"))
	return PhaseRunMetadata{
		Issue:        issue,
		Phase:        phase,
		StartedAt:    started.UTC().Format(time.RFC3339Nano),
		Status:       "running",
		Logs:         PhaseRunLogs{Stdout: filepath.ToSlash(filepath.Join(baseRel, "stdout.log")), Stderr: filepath.ToSlash(filepath.Join(baseRel, "stderr.log"))},
		MetadataPath: metadataRel,
	}
}

// SavePhaseRunMetadata writes metadata atomically enough for local CLI use.
func SavePhaseRunMetadata(targetRoot string, metadata PhaseRunMetadata) error {
	path := filepath.Join(targetRoot, filepath.FromSlash(metadata.MetadataPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create metadata dir: %w", err)
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal phase run metadata: %w", err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp metadata: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace metadata: %w", err)
	}
	return nil
}

// PhaseRunLogPath returns an absolute path for a log entry referenced by metadata.
func PhaseRunLogPath(targetRoot string, relPath string) string {
	return filepath.Join(targetRoot, filepath.FromSlash(relPath))
}

// RelativizePath returns a slash-separated path relative to targetRoot when possible.
func RelativizePath(targetRoot string, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	rel, err := filepath.Rel(targetRoot, path)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

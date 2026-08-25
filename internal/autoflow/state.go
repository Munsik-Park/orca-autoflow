package autoflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State records Orca's view of an AutoFlow issue pipeline.
type State struct {
	Issue              int               `json:"issue"`
	Active             bool              `json:"active"`
	Phase              string            `json:"phase"`
	LastCompletedPhase string            `json:"last_completed_phase,omitempty"`
	Adapter            string            `json:"adapter"`
	Model              string            `json:"model,omitempty"`
	Artifacts          map[string]string `json:"artifacts"`
	UpdatedAt          string            `json:"updated_at"`
}

// StatePath returns the path for Orca-owned AutoFlow state.
func StatePath(targetRoot string, issue int) string {
	return filepath.Join(targetRoot, ".autoflow", fmt.Sprintf("issue-%d-orca.json", issue))
}

// SaveState writes the issue state atomically enough for local CLI use.
func SaveState(targetRoot string, state State) error {
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if state.Artifacts == nil {
		state.Artifacts = map[string]string{}
	}
	path := StatePath(targetRoot, state.Issue)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}

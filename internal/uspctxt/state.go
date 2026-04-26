package uspctxt

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// State is the high-water-mark per CLI persisted between bridge runs.
//
// Schema mirrors spec §4.5. Atomic write via tmpfile + rename keeps
// the file consistent if the bridge is killed mid-update.
type State struct {
	PerCLI  map[string]CLIState `json:"per_cli"`
	Version int                 `json:"version"`
}

// CLIState records the latest successfully-ingested session start
// time for one CLI.
type CLIState struct {
	LastStartedAt time.Time `json:"last_started_at"`
}

// SchemaVersion is the on-disk State version. Increment on
// breaking schema changes.
const SchemaVersion = 1

// DefaultStatePath is the spec-defined location for the bridge
// state file. Returns absolute path under the user's home.
func DefaultStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("uspctxt: home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "usp-ctxt", "last_run.json"), nil
}

// LoadState reads the state file at path. Missing file returns an
// empty State with Version = SchemaVersion (default = epoch hwm
// for every CLI).
func LoadState(path string) (*State, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &State{
				PerCLI:  map[string]CLIState{},
				Version: SchemaVersion,
			}, nil
		}
		return nil, fmt.Errorf("uspctxt: read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("uspctxt: decode state: %w", err)
	}
	if s.PerCLI == nil {
		s.PerCLI = map[string]CLIState{}
	}
	if s.Version == 0 {
		s.Version = SchemaVersion
	}
	return &s, nil
}

// SaveState writes the state to path atomically (tmpfile + rename).
func SaveState(path string, s *State) error {
	if s.Version == 0 {
		s.Version = SchemaVersion
	}
	if s.PerCLI == nil {
		s.PerCLI = map[string]CLIState{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("uspctxt: mkdir state dir: %w", err)
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("uspctxt: encode state: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "last_run-*.json")
	if err != nil {
		return fmt.Errorf("uspctxt: tmpfile: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("uspctxt: write state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("uspctxt: close state: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("uspctxt: rename state: %w", err)
	}
	return nil
}

// HWM returns the high-water-mark for a given CLI; zero time if
// the CLI has no recorded run yet.
func (s *State) HWM(cli string) time.Time {
	if s == nil || s.PerCLI == nil {
		return time.Time{}
	}
	return s.PerCLI[cli].LastStartedAt
}

// Advance updates the hwm for cli to t when t is strictly later than
// the existing value. Returns true if the hwm moved.
func (s *State) Advance(cli string, t time.Time) bool {
	if s.PerCLI == nil {
		s.PerCLI = map[string]CLIState{}
	}
	cur := s.PerCLI[cli].LastStartedAt
	if !t.After(cur) {
		return false
	}
	s.PerCLI[cli] = CLIState{LastStartedAt: t}
	return true
}

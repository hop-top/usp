package uspctxt

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadState_MissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadState(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("LoadState missing: %v", err)
	}
	if s == nil || s.PerCLI == nil {
		t.Fatalf("missing state should return empty State, got %+v", s)
	}
	if s.Version != SchemaVersion {
		t.Errorf("expected default Version=%d; got %d", SchemaVersion, s.Version)
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last_run.json")

	s := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}
	t1 := time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	s.Advance("claude", t1)
	s.Advance("codex", t2)

	if err := SaveState(path, s); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if !loaded.HWM("claude").Equal(t1) {
		t.Errorf("claude hwm: want %v got %v", t1, loaded.HWM("claude"))
	}
	if !loaded.HWM("codex").Equal(t2) {
		t.Errorf("codex hwm: want %v got %v", t2, loaded.HWM("codex"))
	}
}

func TestAdvance_OnlyForwards(t *testing.T) {
	s := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}
	t1 := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	t0 := time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)

	if !s.Advance("claude", t1) {
		t.Fatal("first advance should move hwm")
	}
	if s.Advance("claude", t0) {
		t.Errorf("backwards advance should be rejected")
	}
	if !s.HWM("claude").Equal(t1) {
		t.Errorf("hwm should remain at %v; got %v", t1, s.HWM("claude"))
	}
	if s.Advance("claude", t1) {
		t.Errorf("equal advance should be rejected (strictly-after semantics)")
	}
}

func TestSaveState_AtomicCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "last_run.json")
	s := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}
	s.Advance("claude", time.Now())
	if err := SaveState(path, s); err != nil {
		t.Fatalf("SaveState should mkdir parents: %v", err)
	}
	if _, err := LoadState(path); err != nil {
		t.Errorf("expected loadable state after save: %v", err)
	}
}

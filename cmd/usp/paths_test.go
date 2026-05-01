package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLineageDBPathHonorsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/x")
	got, err := lineageDBPath()
	if err != nil {
		t.Fatalf("lineageDBPath: %v", err)
	}
	want := filepath.Join("/tmp/x", "usp", "sessions.db")
	if got != want {
		t.Errorf("lineageDBPath = %q, want %q", got, want)
	}
}

func TestIndexDBPathHonorsXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/d")
	got, err := indexDBPath()
	if err != nil {
		t.Fatalf("indexDBPath: %v", err)
	}
	want := filepath.Join("/tmp/d", "usp", "index.db")
	if got != want {
		t.Errorf("indexDBPath = %q, want %q", got, want)
	}
}

func TestLineageDBPathFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	got, err := lineageDBPath()
	if err != nil {
		t.Fatalf("lineageDBPath: %v", err)
	}
	// Platform-specific: linux ends usp/sessions.db; darwin ends
	// usp/state/sessions.db. Both contain "usp" + "sessions.db".
	if !strings.Contains(got, "usp") || !strings.HasSuffix(got, "sessions.db") {
		t.Errorf("lineageDBPath fallback = %q, missing usp + sessions.db", got)
	}
}

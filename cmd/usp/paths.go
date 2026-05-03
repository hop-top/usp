package main

import (
	"fmt"
	"path/filepath"

	"hop.top/kit/go/core/xdg"
)

// indexDBPath returns the canonical project index DB path under XDG data.
func indexDBPath() (string, error) {
	dir, err := xdg.DataDir("usp")
	if err != nil {
		return "", fmt.Errorf("data dir: %w", err)
	}
	return filepath.Join(dir, "index.db"), nil
}

// lineageDBPath returns the canonical lineage DB path under XDG state.
func lineageDBPath() (string, error) {
	dir, err := xdg.StateDir("usp")
	if err != nil {
		return "", fmt.Errorf("state dir: %w", err)
	}
	return filepath.Join(dir, "sessions.db"), nil
}

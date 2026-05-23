package main

import (
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

func TestMostRecentClosedPrefersEndedAt(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	oldEnd := now.Add(-2 * time.Hour)
	newEnd := now.Add(-time.Hour)
	got := mostRecentClosed([]session.Session{
		{ID: "newer-start-active", CLI: uxp.CLICodex, StartedAt: now},
		{ID: "old-closed", CLI: uxp.CLICodex, StartedAt: now.Add(-4 * time.Hour), EndedAt: &oldEnd},
		{ID: "new-closed", CLI: uxp.CLICodex, StartedAt: now.Add(-3 * time.Hour), EndedAt: &newEnd},
	})
	if got.ID != "new-closed" {
		t.Fatalf("mostRecentClosed = %q, want new-closed", got.ID)
	}
}

func TestMostRecentClosedFallsBackToFirstWhenNoEndedSessions(t *testing.T) {
	got := mostRecentClosed([]session.Session{
		{ID: "first", CLI: uxp.CLICodex},
		{ID: "second", CLI: uxp.CLICodex},
	})
	if got.ID != "first" {
		t.Fatalf("mostRecentClosed = %q, want first", got.ID)
	}
}

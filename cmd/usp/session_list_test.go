package main

import (
	"testing"
	"time"
)

func TestSessionListCmd_Flags(t *testing.T) {
	cmd := sessionListCmd()

	if cmd.Use != "list" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "list")
	}

	// --format is inherited from root persistent flag.
	for _, name := range []string{"project", "tool", "since", "limit"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}

	lim, err := cmd.Flags().GetInt("limit")
	if err != nil {
		t.Fatal(err)
	}
	if lim != 20 {
		t.Errorf("limit default = %d, want 20", lim)
	}
}

func TestSessionListCmd_Wired(t *testing.T) {
	parent := sessionCmd(nil)
	found := false
	for _, sub := range parent.Commands() {
		if sub.Use == "list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("session list not wired into session parent command")
	}
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		in  string
		max int
		out string
	}{
		{"short", 12, "short"},
		{"abcdefghijklmnop", 12, "abcdefghijkl…"},
	}
	for _, tt := range tests {
		got := truncateID(tt.in, tt.max)
		if got != tt.out {
			t.Errorf("truncateID(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.out)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
		{now.Add(-48 * time.Hour), "2d ago"},
		{now.Add(-24 * time.Hour), "1d ago"},
	}
	for _, tt := range tests {
		got := relativeTime(tt.t)
		if got != tt.want {
			t.Errorf("relativeTime(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

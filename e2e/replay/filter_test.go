package replay

import (
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

// --- stub adapter for FilterAdapters tests ---

type stubAdapter struct{ cli uxp.CLIName }

func (s *stubAdapter) CLI() uxp.CLIName                                { return s.cli }
func (s *stubAdapter) ProjectKey(string) string                        { return "" }
func (s *stubAdapter) Detect() (*uxp.DetectResult, error)              { return nil, nil }
func (s *stubAdapter) Capabilities() uxp.CapabilityMap                 { return nil }
func (s *stubAdapter) ListSessions(string) ([]session.Session, error)  { return nil, nil }
func (s *stubAdapter) GetSession(string) (*session.Session, error)     { return nil, nil }
func (s *stubAdapter) StreamTurns(string) (<-chan session.Turn, error) { return nil, nil }

func mkSession(
	id string, cli uxp.CLIName, project string, ago time.Duration,
) session.Session {
	return session.Session{
		ID:         id,
		CLI:        cli,
		ProjectCwd: project,
		StartedAt:  time.Now().Add(-ago),
		TurnCount:  3,
	}
}

// ====================== Tests ======================

func TestFilterBySince(t *testing.T) {
	now := time.Now()
	sessions := []session.Session{
		mkSession("a", uxp.CLIClaude, "/tmp/a", 1*time.Hour),
		mkSession("b", uxp.CLICodex, "/tmp/b", 2*time.Hour),
		mkSession("c", uxp.CLIGemini, "/tmp/a", 25*time.Hour),
		mkSession("d", uxp.CLIOpenCode, "/tmp/b", 48*time.Hour),
	}

	// 24h window: should keep a + b only.
	got := sessionutil.FilterSince(
		append([]session.Session{}, sessions...),
		now.Add(-24*time.Hour),
	)
	if len(got) != 2 {
		t.Fatalf("24h filter: got %d sessions, want 2", len(got))
	}
	for _, s := range got {
		if s.ID != "a" && s.ID != "b" {
			t.Errorf("24h filter: unexpected session %q", s.ID)
		}
	}

	// 90m window: should keep a only.
	got = sessionutil.FilterSince(
		append([]session.Session{}, sessions...),
		now.Add(-90*time.Minute),
	)
	if len(got) != 1 {
		t.Fatalf("90m filter: got %d sessions, want 1", len(got))
	}
	if got[0].ID != "a" {
		t.Errorf("90m filter: got %q, want a", got[0].ID)
	}

	// Zero time (no filter): all returned.
	got = sessionutil.FilterSince(
		append([]session.Session{}, sessions...), time.Time{},
	)
	if len(got) != 4 {
		t.Fatalf("zero filter: got %d, want 4", len(got))
	}
}

func TestFilterByTool(t *testing.T) {
	all := map[string]session.SessionAdapter{
		"claude":   &stubAdapter{uxp.CLIClaude},
		"codex":    &stubAdapter{uxp.CLICodex},
		"gemini":   &stubAdapter{uxp.CLIGemini},
		"opencode": &stubAdapter{uxp.CLIOpenCode},
	}

	got := sessionutil.FilterAdapters(all, "claude")
	if len(got) != 1 {
		t.Fatalf("claude filter: got %d, want 1", len(got))
	}

	got = sessionutil.FilterAdapters(all, "")
	if len(got) != 4 {
		t.Fatalf("empty filter: got %d, want 4", len(got))
	}

	got = sessionutil.FilterAdapters(all, "unknown")
	if got != nil {
		t.Error("unknown tool: expected nil")
	}
}

func TestSortAndLimit(t *testing.T) {
	sessions := []session.Session{
		mkSession("e", uxp.CLIClaude, "/tmp/x", 10*time.Hour),
		mkSession("a", uxp.CLICodex, "/tmp/x", 1*time.Hour),
		mkSession("c", uxp.CLIGemini, "/tmp/x", 5*time.Hour),
		mkSession("b", uxp.CLIOpenCode, "/tmp/x", 3*time.Hour),
		mkSession("d", uxp.CLIClaude, "/tmp/x", 7*time.Hour),
	}

	got := sessionutil.SortAndLimit(
		append([]session.Session{}, sessions...), 3,
	)
	if len(got) != 3 {
		t.Fatalf("limit 3: got %d, want 3", len(got))
	}
	wantOrder := []string{"a", "b", "c"}
	for i, id := range wantOrder {
		if got[i].ID != id {
			t.Errorf("pos %d: got %q, want %q", i, got[i].ID, id)
		}
	}

	got = sessionutil.SortAndLimit(
		append([]session.Session{}, sessions...), 0,
	)
	if len(got) != 5 {
		t.Fatalf("limit 0: got %d, want 5", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].StartedAt.After(got[i-1].StartedAt) {
			t.Errorf("sort order broken at %d", i)
		}
	}
}

func TestParseSince(t *testing.T) {
	for _, tc := range []struct {
		input string
		maxD  time.Duration
	}{
		{"7d", 7*24*time.Hour + time.Minute},
		{"24h", 24*time.Hour + time.Minute},
		{"30m", 30*time.Minute + time.Minute},
	} {
		got, err := sessionutil.ParseSince(tc.input)
		if err != nil {
			t.Errorf("ParseSince(%q): %v", tc.input, err)
			continue
		}
		elapsed := time.Since(got)
		if elapsed > tc.maxD || elapsed < 0 {
			t.Errorf("ParseSince(%q): elapsed %v out of range",
				tc.input, elapsed)
		}
	}

	got, err := sessionutil.ParseSince("2026-04-01")
	if err != nil {
		t.Fatalf("date parse: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 4 || got.Day() != 1 {
		t.Errorf("date parse: got %v", got)
	}

	got, err = sessionutil.ParseSince("")
	if err != nil {
		t.Fatalf("empty parse: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("empty parse: got %v, want zero", got)
	}

	_, err = sessionutil.ParseSince("not-a-date")
	if err == nil {
		t.Error("invalid input: expected error")
	}
}

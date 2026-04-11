package replay

import (
	"errors"
	"sort"
	"strconv"
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// filterSince mirrors cmd/usp.filterSince (package main; not importable).
func filterSince(ss []session.Session, since time.Time) []session.Session {
	if since.IsZero() {
		return ss
	}
	filtered := ss[:0]
	for _, s := range ss {
		if !s.StartedAt.Before(since) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// sortAndLimit mirrors cmd/usp.sortAndLimit.
func sortAndLimit(ss []session.Session, limit int) []session.Session {
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].StartedAt.After(ss[j].StartedAt)
	})
	if limit > 0 && len(ss) > limit {
		ss = ss[:limit]
	}
	return ss
}

// parseSince mirrors cmd/usp.parseSince. Mirrored because cmd/usp
// is package main (not importable). TODO: extract to internal/cmdhelpers
// to avoid drift between this copy and the real implementation.
func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if len(s) >= 2 {
		unit := s[len(s)-1]
		val := s[:len(s)-1]
		n, err := strconv.Atoi(val)
		if err == nil {
			switch unit {
			case 'd':
				return time.Now().Add(-time.Duration(n) * 24 * time.Hour), nil
			case 'h':
				return time.Now().Add(-time.Duration(n) * time.Hour), nil
			case 'm':
				return time.Now().Add(-time.Duration(n) * time.Minute), nil
			}
		}
	}
	for _, layout := range []string{
		"2006-01-02",
		"2006-01-02T15:04:05",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errInvalid
}

// filterAdapters mirrors cmd/usp.filterAdapters.
func filterAdapters(
	all map[string]session.SessionAdapter, tool string,
) map[string]session.SessionAdapter {
	if tool == "" {
		return all
	}
	a, ok := all[tool]
	if !ok {
		return nil
	}
	return map[string]session.SessionAdapter{tool: a}
}

var errInvalid = errors.New("invalid since value")

// --- stub adapter for filterAdapters tests ---

type stubAdapter struct{ cli uxp.CLIName }

func (s *stubAdapter) CLI() uxp.CLIName                              { return s.cli }
func (s *stubAdapter) ProjectKey(string) string                       { return "" }
func (s *stubAdapter) Detect() (*uxp.DetectResult, error)            { return nil, nil }
func (s *stubAdapter) Capabilities() uxp.CapabilityMap               { return nil }
func (s *stubAdapter) ListSessions(string) ([]session.Session, error) { return nil, nil }
func (s *stubAdapter) GetSession(string) (*session.Session, error)   { return nil, nil }
func (s *stubAdapter) StreamTurns(string) (<-chan session.Turn, error) {
	return nil, nil
}

// --- helpers ---

func mkSession(id string, cli uxp.CLIName, project string, ago time.Duration) session.Session {
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
	got := filterSince(append([]session.Session{}, sessions...), now.Add(-24*time.Hour))
	if len(got) != 2 {
		t.Fatalf("24h filter: got %d sessions, want 2", len(got))
	}
	for _, s := range got {
		if s.ID != "a" && s.ID != "b" {
			t.Errorf("24h filter: unexpected session %q", s.ID)
		}
	}

	// 90m window: should keep a only (b is 2h ago, outside 90m).
	got = filterSince(
		append([]session.Session{}, sessions...), now.Add(-90*time.Minute),
	)
	if len(got) != 1 {
		t.Fatalf("90m filter: got %d sessions, want 1", len(got))
	}
	if got[0].ID != "a" {
		t.Errorf("90m filter: got %q, want a", got[0].ID)
	}

	// Zero time (no filter): all returned.
	got = filterSince(append([]session.Session{}, sessions...), time.Time{})
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

	// Filter to claude only.
	got := filterAdapters(all, "claude")
	if len(got) != 1 {
		t.Fatalf("claude filter: got %d adapters, want 1", len(got))
	}
	if _, ok := got["claude"]; !ok {
		t.Error("claude filter: missing claude key")
	}

	// Empty tool = all adapters.
	got = filterAdapters(all, "")
	if len(got) != 4 {
		t.Fatalf("empty filter: got %d adapters, want 4", len(got))
	}

	// Unknown tool = nil.
	got = filterAdapters(all, "unknown")
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

	// Limit 3: most recent 3, sorted desc.
	got := sortAndLimit(append([]session.Session{}, sessions...), 3)
	if len(got) != 3 {
		t.Fatalf("limit 3: got %d, want 3", len(got))
	}
	wantOrder := []string{"a", "b", "c"}
	for i, id := range wantOrder {
		if got[i].ID != id {
			t.Errorf("limit 3 pos %d: got %q, want %q", i, got[i].ID, id)
		}
	}

	// Limit 0: all returned, still sorted.
	got = sortAndLimit(append([]session.Session{}, sessions...), 0)
	if len(got) != 5 {
		t.Fatalf("limit 0: got %d, want 5", len(got))
	}
	// Verify descending order.
	for i := 1; i < len(got); i++ {
		if got[i].StartedAt.After(got[i-1].StartedAt) {
			t.Errorf("sort order broken at %d: %v after %v",
				i, got[i].StartedAt, got[i-1].StartedAt)
		}
	}
}

func TestParseSince(t *testing.T) {
	// Duration shorthand.
	for _, tc := range []struct {
		input string
		maxD  time.Duration
	}{
		{"7d", 7*24*time.Hour + time.Minute},
		{"24h", 24*time.Hour + time.Minute},
		{"30m", 30*time.Minute + time.Minute},
	} {
		got, err := parseSince(tc.input)
		if err != nil {
			t.Errorf("parseSince(%q): %v", tc.input, err)
			continue
		}
		elapsed := time.Since(got)
		if elapsed > tc.maxD || elapsed < 0 {
			t.Errorf("parseSince(%q): elapsed %v out of range", tc.input, elapsed)
		}
	}

	// Date formats.
	got, err := parseSince("2026-04-01")
	if err != nil {
		t.Fatalf("date parse: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 4 || got.Day() != 1 {
		t.Errorf("date parse: got %v", got)
	}

	got, err = parseSince("2026-04-01T10:00:00")
	if err != nil {
		t.Fatalf("datetime parse: %v", err)
	}
	if got.Hour() != 10 {
		t.Errorf("datetime parse: hour = %d, want 10", got.Hour())
	}

	// Empty string: zero time.
	got, err = parseSince("")
	if err != nil {
		t.Fatalf("empty parse: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("empty parse: got %v, want zero", got)
	}

	// Invalid input: error.
	_, err = parseSince("not-a-date")
	if err == nil {
		t.Error("invalid input: expected error")
	}
}

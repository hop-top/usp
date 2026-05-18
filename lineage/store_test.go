package lineage

import (
	"path/filepath"
	"testing"

	"hop.top/kit/go/core/uxp"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sessions.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetSession(t *testing.T) {
	s := openTestStore(t)

	if err := s.CreateSession("usp-001", "/tmp/project"); err != nil {
		t.Fatal(err)
	}

	sess, err := s.GetSession("usp-001")
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID != "usp-001" {
		t.Errorf("ID = %q, want %q", sess.ID, "usp-001")
	}
	if sess.ProjectCwd != "/tmp/project" {
		t.Errorf("ProjectCwd = %q", sess.ProjectCwd)
	}
	if len(sess.Segments) != 0 {
		t.Errorf("Segments = %d, want 0", len(sess.Segments))
	}
}

func TestAddSegmentsAndLineage(t *testing.T) {
	s := openTestStore(t)

	if err := s.CreateSession("usp-002", "/tmp/srv"); err != nil {
		t.Fatal(err)
	}

	// Segment 1: Claude
	if err := s.AddSegment("usp-002", uxp.CLIClaude, "claude-abc", 10); err != nil {
		t.Fatal(err)
	}
	// Segment 2: Codex
	if err := s.AddSegment("usp-002", uxp.CLICodex, "codex-def", 5); err != nil {
		t.Fatal(err)
	}
	// Segment 3: Gemini
	if err := s.AddSegment("usp-002", uxp.CLIGemini, "gemini-ghi", 3); err != nil {
		t.Fatal(err)
	}

	sess, err := s.GetSession("usp-002")
	if err != nil {
		t.Fatal(err)
	}

	if len(sess.Segments) != 3 {
		t.Fatalf("Segments = %d, want 3", len(sess.Segments))
	}
	if sess.TurnCount != 18 {
		t.Errorf("TurnCount = %d, want 18", sess.TurnCount)
	}
	if sess.CLI != uxp.CLIClaude {
		t.Errorf("CLI = %q, want %q (first segment)", sess.CLI, uxp.CLIClaude)
	}

	// Verify segment order.
	expected := []struct {
		cli      string
		nativeID string
		turns    int
	}{
		{uxp.CLIClaude, "claude-abc", 10},
		{uxp.CLICodex, "codex-def", 5},
		{uxp.CLIGemini, "gemini-ghi", 3},
	}
	for i, e := range expected {
		seg := sess.Segments[i]
		if seg.CLI != e.cli {
			t.Errorf("seg[%d].CLI = %q, want %q", i, seg.CLI, e.cli)
		}
		if seg.NativeID != e.nativeID {
			t.Errorf("seg[%d].NativeID = %q, want %q", i, seg.NativeID, e.nativeID)
		}
		if seg.TurnCount != e.turns {
			t.Errorf("seg[%d].TurnCount = %d, want %d", i, seg.TurnCount, e.turns)
		}
	}
}

func TestListSessionsByCwd(t *testing.T) {
	s := openTestStore(t)

	s.CreateSession("s1", "/tmp/a")
	s.CreateSession("s2", "/tmp/b")
	s.CreateSession("s3", "/tmp/a")

	all, err := s.ListSessions("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("ListSessions('') = %d, want 3", len(all))
	}

	filtered, err := s.ListSessions("/tmp/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 {
		t.Errorf("ListSessions('/tmp/a') = %d, want 2", len(filtered))
	}
}

func TestGetSessionNotFound(t *testing.T) {
	s := openTestStore(t)

	_, err := s.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestSequentialSegmentOrdering(t *testing.T) {
	s := openTestStore(t)

	s.CreateSession("s4", "/tmp/x")
	s.AddSegment("s4", uxp.CLIClaude, "n1", 5)
	s.AddSegment("s4", uxp.CLICodex, "n2", 3)
	s.AddSegment("s4", uxp.CLIGemini, "n3", 7)
	s.AddSegment("s4", uxp.CLIOpenCode, "n4", 2)

	sess, _ := s.GetSession("s4")
	if len(sess.Segments) != 4 {
		t.Fatalf("got %d segments, want 4", len(sess.Segments))
	}

	clis := []string{uxp.CLIClaude, uxp.CLICodex, uxp.CLIGemini, uxp.CLIOpenCode}
	for i, want := range clis {
		if sess.Segments[i].CLI != want {
			t.Errorf("seg[%d].CLI = %q, want %q", i, sess.Segments[i].CLI, want)
		}
	}
	if sess.TurnCount != 17 {
		t.Errorf("TurnCount = %d, want 17", sess.TurnCount)
	}
}

package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/lineage"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

const testCwd = "/tmp/cross-resume-project"

func seedTurns() []session.Turn {
	return []session.Turn{
		{Role: session.RoleUser, Content: "create HTTP server",
			Timestamp: time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)},
		{Role: session.RoleAssistant, Content: "here's the code",
			Timestamp: time.Date(2026, 4, 11, 9, 0, 5, 0, time.UTC)},
		{Role: session.RoleUser, Content: "add error handling",
			Timestamp: time.Date(2026, 4, 11, 9, 1, 0, 0, time.UTC)},
	}
}

// setupGemini writes a minimal projects.json so ProjectKey resolves.
func setupGemini(t *testing.T, home string) {
	t.Helper()
	p := filepath.Join(home, ".gemini", "projects.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{
		"projects": map[string]string{testCwd: "test-project"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// drainTurns collects all turns from a channel.
func drainTurns(ch <-chan session.Turn) []session.Turn {
	var out []session.Turn
	for t := range ch {
		out = append(out, t)
	}
	return out
}

// injectAndVerify is the shared inject→stream→verify chain used
// by both tests. Returns native IDs for claude, codex, gemini.
func injectAndVerify(t *testing.T, home string) (string, string, string) {
	t.Helper()
	turns := seedTurns()

	// --- Claude ---
	claudeA := claude.New(claude.WithHomeDir(home))
	claudeID, err := claudeA.InjectSession(testCwd, turns)
	if err != nil {
		t.Fatalf("claude inject: %v", err)
	}
	ch, err := claudeA.StreamTurns(claudeID)
	if err != nil {
		t.Fatalf("claude stream: %v", err)
	}
	claudeTurns := drainTurns(ch)
	if len(claudeTurns) != 3 {
		t.Fatalf("claude turns = %d, want 3", len(claudeTurns))
	}

	// --- Codex ---
	restoreSR := codex.SetSessionsRoot(
		filepath.Join(home, ".codex", "sessions"),
	)
	restoreCR := codex.SetCodexRoot(filepath.Join(home, ".codex"))
	t.Cleanup(restoreSR)
	t.Cleanup(restoreCR)

	codexA := &codex.Adapter{}
	codexID, err := codexA.InjectSession(testCwd, claudeTurns)
	if err != nil {
		t.Fatalf("codex inject: %v", err)
	}
	ch, err = codexA.StreamTurns(codexID)
	if err != nil {
		t.Fatalf("codex stream: %v", err)
	}
	codexTurns := drainTurns(ch)
	if len(codexTurns) != 3 {
		t.Fatalf("codex turns = %d, want 3", len(codexTurns))
	}

	// --- Gemini ---
	setupGemini(t, home)
	geminiA := &gemini.Adapter{HomeDir: home}
	geminiID, err := geminiA.InjectSession(testCwd, codexTurns)
	if err != nil {
		t.Fatalf("gemini inject: %v", err)
	}
	ch, err = geminiA.StreamTurns(geminiID)
	if err != nil {
		t.Fatalf("gemini stream: %v", err)
	}
	geminiTurns := drainTurns(ch)
	if len(geminiTurns) != 3 {
		t.Fatalf("gemini turns = %d, want 3", len(geminiTurns))
	}

	return claudeID, codexID, geminiID
}

// buildLineage creates a lineage store and registers 3 segments.
func buildLineage(
	t *testing.T, dir string,
	claudeID, codexID, geminiID string,
) *lineage.Store {
	t.Helper()
	store, err := lineage.Open(filepath.Join(dir, "lineage.db"))
	if err != nil {
		t.Fatalf("lineage open: %v", err)
	}

	const uspID = "usp-cross-resume-001"
	if err := store.CreateSession(uspID, testCwd); err != nil {
		t.Fatalf("create session: %v", err)
	}
	for _, seg := range []struct {
		cli uxp.CLIName
		id  string
	}{
		{uxp.CLIClaude, claudeID},
		{uxp.CLICodex, codexID},
		{uxp.CLIGemini, geminiID},
	} {
		if err := store.AddSegment(uspID, seg.cli, seg.id, 3); err != nil {
			t.Fatalf("add segment %s: %v", seg.cli, err)
		}
	}
	return store
}

// verifyLineage asserts segment count, total turns, CLI ordering.
func verifyLineage(
	t *testing.T, store *lineage.Store,
	claudeID, codexID, geminiID string,
) {
	t.Helper()
	const uspID = "usp-cross-resume-001"

	sess, err := store.GetSession(uspID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(sess.Segments) != 3 {
		t.Fatalf("segments = %d, want 3", len(sess.Segments))
	}
	if sess.TurnCount != 9 {
		t.Errorf("total turns = %d, want 9", sess.TurnCount)
	}

	want := []struct {
		cli uxp.CLIName
		id  string
	}{
		{uxp.CLIClaude, claudeID},
		{uxp.CLICodex, codexID},
		{uxp.CLIGemini, geminiID},
	}
	for i, w := range want {
		seg := sess.Segments[i]
		if seg.CLI != w.cli {
			t.Errorf("segment[%d].CLI = %q, want %q", i, seg.CLI, w.cli)
		}
		if seg.NativeID != w.id {
			t.Errorf("segment[%d].NativeID = %q, want %q", i, seg.NativeID, w.id)
		}
		if seg.TurnCount != 3 {
			t.Errorf("segment[%d].TurnCount = %d, want 3", i, seg.TurnCount)
		}
	}
}

// TestCrossCliResume verifies the full Claude → Codex → Gemini chain
// with lineage tracking across all three adapters.
func TestCrossCliResume(t *testing.T) {
	home := t.TempDir()
	claudeID, codexID, geminiID := injectAndVerify(t, home)

	store := buildLineage(t, home, claudeID, codexID, geminiID)
	defer store.Close()

	verifyLineage(t, store, claudeID, codexID, geminiID)
}

// TestCrossCliResumeLineagePersistence verifies lineage survives
// store close and reopen at the same path.
func TestCrossCliResumeLineagePersistence(t *testing.T) {
	home := t.TempDir()
	claudeID, codexID, geminiID := injectAndVerify(t, home)

	dbPath := filepath.Join(home, "lineage.db")

	// Build lineage, then close.
	store := buildLineage(t, home, claudeID, codexID, geminiID)
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopen at same path.
	store2, err := lineage.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store2.Close()

	verifyLineage(t, store2, claudeID, codexID, geminiID)
}

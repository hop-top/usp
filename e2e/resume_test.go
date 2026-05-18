package e2e

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/adapters/opencode"
	"hop.top/usp/lineage"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

// TestCrossCliResumeRoundTrip verifies the full chain:
// Claude → Codex → Gemini with lineage tracking.
func TestCrossCliResumeRoundTrip(t *testing.T) {
	home := t.TempDir()

	// --- 1. Seed Claude with 3 turns via InjectSession ---
	claudeA := claude.New(claude.WithHomeDir(home))

	seedTurns := []session.Turn{
		{Role: session.RoleUser, Content: "create HTTP server",
			Timestamp: time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)},
		{Role: session.RoleAssistant, Content: "here's the code...",
			Timestamp: time.Date(2026, 4, 11, 9, 0, 5, 0, time.UTC)},
		{Role: session.RoleUser, Content: "add error handling",
			Timestamp: time.Date(2026, 4, 11, 9, 1, 0, 0, time.UTC)},
	}

	claudeID, err := claudeA.InjectSession(fixtureCwd, seedTurns)
	if err != nil {
		t.Fatalf("claude InjectSession: %v", err)
	}
	if claudeID == "" {
		t.Fatal("claude InjectSession returned empty ID")
	}

	// --- 2. Read turns back via Claude StreamTurns ---
	ch, err := claudeA.StreamTurns(claudeID)
	if err != nil {
		t.Fatalf("claude StreamTurns: %v", err)
	}
	var claudeTurns []session.Turn
	for turn := range ch {
		claudeTurns = append(claudeTurns, turn)
	}
	if len(claudeTurns) != 3 {
		t.Fatalf("claude turns = %d, want 3", len(claudeTurns))
	}

	// --- 3. Inject into Codex ---
	restoreSR := codex.SetSessionsRoot(
		filepath.Join(home, ".codex", "sessions"),
	)
	restoreCR := codex.SetCodexRoot(filepath.Join(home, ".codex"))
	t.Cleanup(restoreSR)
	t.Cleanup(restoreCR)

	codexA := &codex.Adapter{}
	codexID, err := codexA.InjectSession(fixtureCwd, claudeTurns)
	if err != nil {
		t.Fatalf("codex InjectSession: %v", err)
	}
	if codexID == "" {
		t.Fatal("codex InjectSession returned empty ID")
	}

	// Verify Codex session exists.
	codexSess, err := codexA.GetSession(codexID)
	if err != nil {
		t.Fatalf("codex GetSession: %v", err)
	}
	if codexSess.CLI != uxp.CLICodex {
		t.Errorf("codex CLI = %q, want %q", codexSess.CLI, uxp.CLICodex)
	}

	// --- 4. Read Codex turns, inject into Gemini ---
	codexCh, err := codexA.StreamTurns(codexID)
	if err != nil {
		t.Fatalf("codex StreamTurns: %v", err)
	}
	var codexTurns []session.Turn
	for turn := range codexCh {
		codexTurns = append(codexTurns, turn)
	}
	if len(codexTurns) != 3 {
		t.Fatalf("codex turns = %d, want 3", len(codexTurns))
	}

	// Set up Gemini projects.json so ProjectKey resolves.
	geminiA := &gemini.Adapter{HomeDir: home}
	writeJSON(t, filepath.Join(home, ".gemini", "projects.json"),
		map[string]any{
			"projects": map[string]string{fixtureCwd: "test-project"},
		},
	)

	geminiID, err := geminiA.InjectSession(fixtureCwd, codexTurns)
	if err != nil {
		t.Fatalf("gemini InjectSession: %v", err)
	}
	if geminiID == "" {
		t.Fatal("gemini InjectSession returned empty ID")
	}

	// Verify Gemini session file was created.
	geminiCh, err := geminiA.StreamTurns(geminiID)
	if err != nil {
		t.Fatalf("gemini StreamTurns: %v", err)
	}
	var geminiTurns []session.Turn
	for turn := range geminiCh {
		geminiTurns = append(geminiTurns, turn)
	}
	if len(geminiTurns) != 3 {
		t.Fatalf("gemini turns = %d, want 3", len(geminiTurns))
	}

	// --- 5. Lineage store: track cross-CLI session ---
	storePath := filepath.Join(home, "lineage.db")
	store, err := lineage.Open(storePath)
	if err != nil {
		t.Fatalf("lineage.Open: %v", err)
	}
	defer store.Close()

	uspID := "usp-resume-test-001"
	if err := store.CreateSession(uspID, fixtureCwd); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.AddSegment(uspID, uxp.CLIClaude, claudeID, 3); err != nil {
		t.Fatalf("AddSegment claude: %v", err)
	}
	if err := store.AddSegment(uspID, uxp.CLICodex, codexID, 3); err != nil {
		t.Fatalf("AddSegment codex: %v", err)
	}
	if err := store.AddSegment(uspID, uxp.CLIGemini, geminiID, 3); err != nil {
		t.Fatalf("AddSegment gemini: %v", err)
	}

	// --- 6. Verify lineage ---
	sess, err := store.GetSession(uspID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(sess.Segments) != 3 {
		t.Fatalf("segments = %d, want 3", len(sess.Segments))
	}
	if sess.TurnCount != 9 {
		t.Errorf("total TurnCount = %d, want 9", sess.TurnCount)
	}

	// Verify segment ordering and content.
	wantSegments := []struct {
		cli      uxp.CLIName
		nativeID string
	}{
		{uxp.CLIClaude, claudeID},
		{uxp.CLICodex, codexID},
		{uxp.CLIGemini, geminiID},
	}
	for i, want := range wantSegments {
		seg := sess.Segments[i]
		if seg.CLI != want.cli {
			t.Errorf("segment[%d].CLI = %q, want %q",
				i, seg.CLI, want.cli)
		}
		if seg.NativeID != want.nativeID {
			t.Errorf("segment[%d].NativeID = %q, want %q",
				i, seg.NativeID, want.nativeID)
		}
		if seg.TurnCount != 3 {
			t.Errorf("segment[%d].TurnCount = %d, want 3",
				i, seg.TurnCount)
		}
	}
}

// TestResumeAdapterTypeAssertion verifies all adapters implement
// session.ResumeAdapter.
func TestResumeAdapterTypeAssertion(t *testing.T) {
	home := t.TempDir()

	// OpenCode needs a real SQLite DB for construction.
	dbPath := filepath.Join(home, "oc", "opencode.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE session (
			id TEXT PRIMARY KEY, project_id TEXT,
			title TEXT, directory TEXT,
			created_at INTEGER, updated_at INTEGER)`,
		`CREATE TABLE message (
			id TEXT PRIMARY KEY, session_id TEXT,
			data TEXT, created_at INTEGER)`,
		`CREATE TABLE part (
			id TEXT PRIMARY KEY, message_id TEXT,
			session_id TEXT, data TEXT, created_at INTEGER)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	adapters := []struct {
		name    string
		adapter session.SessionAdapter
	}{
		{"claude", claude.New(claude.WithHomeDir(home))},
		{"codex", &codex.Adapter{}},
		{"gemini", &gemini.Adapter{HomeDir: home}},
		{"opencode", opencode.New(
			opencode.WithDBPath(dbPath),
			opencode.WithHomeDir(home),
		)},
	}

	for _, tt := range adapters {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := tt.adapter.(session.ResumeAdapter)
			if !ok {
				t.Errorf("%s does not implement ResumeAdapter", tt.name)
			}
		})
	}
}

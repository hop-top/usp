package replay

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/adapters/opencode"
	"hop.top/usp/lineage"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

const sharedCwd = "/tmp/shared-project"

// setupSharedHome creates a temp dir with all 4 CLI stores and returns
// adapters for Claude, Codex, Gemini, OpenCode — all rooted at the
// same home directory.
func setupSharedHome(t *testing.T) (
	string,
	*claude.Adapter,
	*codex.Adapter,
	*gemini.Adapter,
	*opencode.Adapter,
) {
	t.Helper()
	home := t.TempDir()

	// Gemini projects.json
	pj := filepath.Join(home, ".gemini", "projects.json")
	if err := os.MkdirAll(filepath.Dir(pj), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{
		"projects": map[string]string{sharedCwd: "shared-project"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pj, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// OpenCode DB
	dbDir := filepath.Join(home, ".local", "share", "opencode")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, title TEXT, directory TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, data TEXT NOT NULL, created_at INTEGER NOT NULL, FOREIGN KEY (session_id) REFERENCES session(id))`,
		`CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT, data TEXT NOT NULL, created_at INTEGER NOT NULL, FOREIGN KEY (message_id) REFERENCES message(id))`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	db.Close()

	// Codex overrides
	restoreSR := codex.SetSessionsRoot(
		filepath.Join(home, ".codex", "sessions"),
	)
	restoreCR := codex.SetCodexRoot(filepath.Join(home, ".codex"))
	t.Cleanup(restoreSR)
	t.Cleanup(restoreCR)

	claudeA := claude.New(claude.WithHomeDir(home))
	codexA := &codex.Adapter{}
	geminiA := &gemini.Adapter{HomeDir: home}
	ocA := opencode.New(opencode.WithHomeDir(home))

	return home, claudeA, codexA, geminiA, ocA
}

// TestMultiCliSessionSharing proves that sessions injected by 4
// different CLIs into the same home dir are independently listable
// and share the same project cwd.
func TestMultiCliSessionSharing(t *testing.T) {
	_, claudeA, codexA, geminiA, ocA := setupSharedHome(t)

	ts := time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)

	claudeTurns := []session.Turn{
		{Role: session.RoleUser, Content: "create HTTP server", Timestamp: ts},
		{Role: session.RoleAssistant, Content: "here is the server code", Timestamp: ts.Add(5 * time.Second)},
		{Role: session.RoleUser, Content: "add graceful shutdown", Timestamp: ts.Add(time.Minute)},
	}
	codexTurns := []session.Turn{
		{Role: session.RoleUser, Content: "add tests for server", Timestamp: ts.Add(2 * time.Minute)},
		{Role: session.RoleAssistant, Content: "tests added", Timestamp: ts.Add(2*time.Minute + 5*time.Second)},
	}
	geminiTurns := []session.Turn{
		{Role: session.RoleUser, Content: "add Dockerfile", Timestamp: ts.Add(3 * time.Minute)},
		{Role: session.RoleAssistant, Content: "Dockerfile created", Timestamp: ts.Add(3*time.Minute + 5*time.Second)},
	}
	ocTurns := []session.Turn{
		{Role: session.RoleUser, Content: "deploy to cloud", Timestamp: ts.Add(4 * time.Minute)},
		{Role: session.RoleAssistant, Content: "deployed successfully", Timestamp: ts.Add(4*time.Minute + 5*time.Second)},
	}

	// Inject into each adapter.
	claudeID, err := claudeA.InjectSession(sharedCwd, claudeTurns)
	if err != nil {
		t.Fatalf("claude inject: %v", err)
	}
	codexID, err := codexA.InjectSession(sharedCwd, codexTurns)
	if err != nil {
		t.Fatalf("codex inject: %v", err)
	}
	geminiID, err := geminiA.InjectSession(sharedCwd, geminiTurns)
	if err != nil {
		t.Fatalf("gemini inject: %v", err)
	}
	ocID, err := ocA.InjectSession(sharedCwd, ocTurns)
	if err != nil {
		t.Fatalf("opencode inject: %v", err)
	}

	// Each adapter lists exactly 1 session for the project.
	for name, tc := range map[string]struct {
		a  session.SessionAdapter
		id string
	}{
		"claude":   {claudeA, claudeID},
		"codex":    {codexA, codexID},
		"opencode": {ocA, ocID},
	} {
		ss, err := tc.a.ListSessions(sharedCwd)
		if err != nil {
			t.Fatalf("%s list: %v", name, err)
		}
		if len(ss) != 1 {
			t.Fatalf("%s sessions = %d, want 1", name, len(ss))
		}
		if ss[0].ID != tc.id {
			t.Errorf("%s id = %q, want %q", name, ss[0].ID, tc.id)
		}
	}

	// Gemini InjectSession writes a .json file, so ListSessions must find it.
	gemSessions, err := geminiA.ListSessions(sharedCwd)
	if err != nil {
		t.Fatalf("gemini list: %v", err)
	}
	if len(gemSessions) != 1 {
		t.Fatalf("gemini sessions = %d, want 1", len(gemSessions))
	}
	if gemSessions[0].ID != geminiID {
		t.Errorf("gemini session ID = %q, want %q", gemSessions[0].ID, geminiID)
	}

	// Aggregate across all adapters (unified view).
	var all []session.Session
	for _, a := range []session.SessionAdapter{claudeA, codexA, geminiA, ocA} {
		ss, err := a.ListSessions(sharedCwd)
		if err != nil {
			t.Fatalf("aggregate list: %v", err)
		}
		all = append(all, ss...)
	}

	if len(all) != 4 {
		t.Fatalf("aggregate sessions = %d, want 4", len(all))
	}

	// Every session shares the same project cwd.
	for _, s := range all {
		if s.ProjectCwd != sharedCwd {
			t.Errorf("session %q cwd = %q, want %q", s.ID, s.ProjectCwd, sharedCwd)
		}
	}

	// CLIs are distinct.
	clis := map[uxp.CLIName]bool{}
	for _, s := range all {
		clis[s.CLI] = true
	}
	if !clis[uxp.CLIClaude] || !clis[uxp.CLICodex] || !clis[uxp.CLIOpenCode] {
		t.Errorf("missing expected CLIs in aggregate: %v", clis)
	}

	// Sort by StartedAt descending.
	sort.Slice(all, func(i, j int) bool {
		return all[i].StartedAt.After(all[j].StartedAt)
	})
	for i := 1; i < len(all); i++ {
		if all[i].StartedAt.After(all[i-1].StartedAt) {
			t.Errorf("sort broken at index %d", i)
		}
	}

	_ = geminiID // used in inject; listed above
}

// TestMultiCliResumeChainSharing proves a conversation that flows
// through 4 CLIs appears as one logical session via lineage.
func TestMultiCliResumeChainSharing(t *testing.T) {
	home, claudeA, codexA, geminiA, ocA := setupSharedHome(t)

	ts := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)
	seed := []session.Turn{
		{Role: session.RoleUser, Content: "create HTTP server", Timestamp: ts},
		{Role: session.RoleAssistant, Content: "server code ready", Timestamp: ts.Add(5 * time.Second)},
		{Role: session.RoleUser, Content: "add middleware", Timestamp: ts.Add(time.Minute)},
	}

	// 1. Inject into Claude.
	claudeID, err := claudeA.InjectSession(sharedCwd, seed)
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

	// 2. Resume into Codex.
	codexID, err := codexA.InjectSession(sharedCwd, claudeTurns)
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

	// 3. Resume into Gemini.
	geminiID, err := geminiA.InjectSession(sharedCwd, codexTurns)
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

	// 4. Resume into OpenCode.
	ocID, err := ocA.InjectSession(sharedCwd, geminiTurns)
	if err != nil {
		t.Fatalf("opencode inject: %v", err)
	}
	ch, err = ocA.StreamTurns(ocID)
	if err != nil {
		t.Fatalf("opencode stream: %v", err)
	}
	ocTurns := drainTurns(ch)
	if len(ocTurns) != 3 {
		t.Fatalf("opencode turns = %d, want 3", len(ocTurns))
	}

	// 5. List across all adapters — 4 native sessions.
	var all []session.Session
	for _, a := range []session.SessionAdapter{claudeA, codexA, geminiA, ocA} {
		ss, err := a.ListSessions(sharedCwd)
		if err != nil {
			t.Fatalf("ListSessions(%s): %v", a.CLI(), err)
		}
		all = append(all, ss...)
	}
	if len(all) != 4 {
		t.Fatalf("total native sessions = %d, want 4", len(all))
	}

	// 6. Build lineage — one logical session with 4 segments.
	store, err := lineage.Open(filepath.Join(home, "lineage.db"))
	if err != nil {
		t.Fatalf("lineage open: %v", err)
	}
	defer store.Close()

	const uspID = "usp-multi-cli-001"
	if err := store.CreateSession(uspID, sharedCwd); err != nil {
		t.Fatalf("create session: %v", err)
	}
	for _, seg := range []struct {
		cli uxp.CLIName
		id  string
	}{
		{uxp.CLIClaude, claudeID},
		{uxp.CLICodex, codexID},
		{uxp.CLIGemini, geminiID},
		{uxp.CLIOpenCode, ocID},
	} {
		if err := store.AddSegment(uspID, seg.cli, seg.id, 3); err != nil {
			t.Fatalf("add segment %s: %v", seg.cli, err)
		}
	}

	// 7. Verify lineage.
	sess, err := store.GetSession(uspID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(sess.Segments) != 4 {
		t.Fatalf("segments = %d, want 4", len(sess.Segments))
	}
	if sess.TurnCount != 12 {
		t.Errorf("total turns = %d, want 12", sess.TurnCount)
	}

	wantOrder := []uxp.CLIName{
		uxp.CLIClaude, uxp.CLICodex, uxp.CLIGemini, uxp.CLIOpenCode,
	}
	for i, w := range wantOrder {
		if sess.Segments[i].CLI != w {
			t.Errorf("segment[%d].CLI = %q, want %q", i, sess.Segments[i].CLI, w)
		}
		if sess.Segments[i].TurnCount != 3 {
			t.Errorf("segment[%d].TurnCount = %d, want 3", i, sess.Segments[i].TurnCount)
		}
	}

	// 8. Content carried through — OpenCode's turns contain
	// the original seed content (possibly lossy but present).
	if ocTurns[0].Content == "" {
		t.Error("opencode first turn has empty content")
	}
	// Verify seed content propagated through the chain.
	foundSeed := false
	for _, turn := range ocTurns {
		if strings.Contains(turn.Content, "create HTTP server") ||
			strings.Contains(turn.Content, "HTTP server") {
			foundSeed = true
			break
		}
	}
	if !foundSeed {
		t.Error("seed content 'HTTP server' not found in opencode turns — propagation failed")
	}
}

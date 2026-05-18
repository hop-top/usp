package replay

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/adapters/opencode"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

const fixtureCwd = "/tmp/replay-project"
const geminiFixtureSessionID = "88888888-8888-4888-8888-888888888888"

func writeJSONL(t *testing.T, path string, events []map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			t.Fatal(err)
		}
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSingleSessionLifecycle(t *testing.T) {
	t.Run("claude", testClaudeLifecycle)
	t.Run("codex", testCodexLifecycle)
	t.Run("gemini", testGeminiLifecycle)
	t.Run("opencode", testOpenCodeLifecycle)
}

func testClaudeLifecycle(t *testing.T) {
	home := t.TempDir()
	a := claude.New(claude.WithHomeDir(home))
	key := a.ProjectKey(fixtureCwd)
	dir := filepath.Join(home, ".claude", "projects", key)

	writeJSONL(t, filepath.Join(dir, "replay-sess-01.jsonl"), []map[string]any{
		{
			"uuid": "r1", "type": "user",
			"timestamp": "2026-04-11T08:00:00Z",
			"cwd":       fixtureCwd, "sessionId": "replay-sess-01",
			"message": map[string]any{"role": "user", "content": "start server"},
		},
		{
			"uuid": "r2", "type": "assistant",
			"timestamp": "2026-04-11T08:00:05Z",
			"sessionId": "replay-sess-01",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "server started on :8080"},
				},
			},
		},
		{
			"uuid": "r3", "type": "user",
			"timestamp": "2026-04-11T08:01:00Z",
			"sessionId": "replay-sess-01",
			"message":   map[string]any{"role": "user", "content": "add TLS"},
		},
	})

	verifyLifecycle(t, a, "replay-sess-01", uxp.CLIClaude, fixtureCwd, 3)
}

func testCodexLifecycle(t *testing.T) {
	home := t.TempDir()
	sessDir := filepath.Join(home, ".codex", "sessions", "2026", "04", "11")

	writeJSONL(t, filepath.Join(sessDir, "replay-codex-01.jsonl"), []map[string]any{
		{
			"timestamp": "2026-04-11T10:00:00Z",
			"type":      "session_meta",
			"payload": map[string]any{
				"id":          "replay-codex-01",
				"timestamp":   "2026-04-11T10:00:00Z",
				"cwd":         fixtureCwd,
				"originator":  "user",
				"cli_version": "0.106.0",
				"source":      "cli",
			},
		},
		{
			"timestamp": "2026-04-11T10:00:01Z",
			"type":      "response_item",
			"payload": map[string]any{
				"type": "message", "role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "init project"},
				},
			},
		},
		{
			"timestamp": "2026-04-11T10:00:02Z",
			"type":      "response_item",
			"payload": map[string]any{
				"type": "message", "role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "project initialized"},
				},
			},
		},
		{
			"timestamp": "2026-04-11T10:00:03Z",
			"type":      "response_item",
			"payload": map[string]any{
				"type": "message", "role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "add CI"},
				},
			},
		},
	})

	restoreSR := codex.SetSessionsRoot(
		filepath.Join(home, ".codex", "sessions"),
	)
	restoreCR := codex.SetCodexRoot(filepath.Join(home, ".codex"))
	t.Cleanup(restoreSR)
	t.Cleanup(restoreCR)

	a := &codex.Adapter{}
	verifyLifecycle(t, a, "replay-codex-01", uxp.CLICodex, fixtureCwd, 3)
}

func testGeminiLifecycle(t *testing.T) {
	home := t.TempDir()
	alias := "replay-project"

	writeJSON(t, filepath.Join(home, ".gemini", "projects.json"),
		map[string]any{
			"projects": map[string]string{fixtureCwd: alias},
		},
	)

	projectDir := filepath.Join(home, ".gemini", "tmp", alias)
	if err := os.MkdirAll(filepath.Join(projectDir, "chats"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(projectDir, ".project_root"), []byte(fixtureCwd), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	writeJSONL(t,
		filepath.Join(projectDir, "chats",
			"session-2026-04-11T10-00-"+geminiFixtureSessionID[:8]+".jsonl"),
		[]map[string]any{
			{
				"sessionId":   geminiFixtureSessionID,
				"projectHash": "hash",
				"startTime":   "2026-04-11T10:00:00Z",
				"lastUpdated": "2026-04-11T10:00:03Z",
				"kind":        "main",
			},
			{
				"id": "g1", "type": "user",
				"timestamp": "2026-04-11T10:00:00Z",
				"content":   []map[string]string{{"text": "scaffold app"}},
			},
			{
				"id": "g2", "type": "gemini",
				"timestamp": "2026-04-11T10:00:02Z",
				"content":   "app scaffolded",
			},
			{
				"id": "g3", "type": "user",
				"timestamp": "2026-04-11T10:00:03Z",
				"content":   []map[string]string{{"text": "add tests"}},
			},
		},
	)

	a := &gemini.Adapter{HomeDir: home}
	verifyLifecycle(t, a, geminiFixtureSessionID, uxp.CLIGemini, fixtureCwd, 3)
}

func testOpenCodeLifecycle(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(
		home, ".local", "share", "opencode", "opencode.db",
	)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, ddl := range []string{
		`CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, title TEXT, directory TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, data TEXT NOT NULL, created_at INTEGER NOT NULL, FOREIGN KEY (session_id) REFERENCES session(id))`,
		`CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, data TEXT NOT NULL, created_at INTEGER NOT NULL, FOREIGN KEY (message_id) REFERENCES message(id))`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}

	a := opencode.New(opencode.WithHomeDir(home))
	projKey := a.ProjectKey(fixtureCwd)
	now := time.Date(2026, 4, 11, 11, 0, 0, 0, time.UTC).UnixMilli()

	_, err = db.Exec(
		`INSERT INTO session (id,project_id,title,directory,created_at,updated_at)
		 VALUES (?,?,?,?,?,?)`,
		"oc-replay-01", projKey, "Replay Session",
		fixtureCwd, now, now+6000,
	)
	if err != nil {
		t.Fatal(err)
	}

	msgs := []struct {
		id   string
		role string
		text string
		ms   int64
	}{
		{"msg-r1", "user", "deploy service", now},
		{"msg-r2", "assistant", "deployed to prod", now + 2000},
		{"msg-r3", "user", "check logs", now + 4000},
	}
	for _, m := range msgs {
		data, err := json.Marshal(map[string]string{
			"role": m.role, "content": m.text,
		})
		if err != nil {
			t.Fatalf("marshal message: %v", err)
		}
		if _, err := db.Exec(
			`INSERT INTO message (id,session_id,data,created_at)
			 VALUES (?,?,?,?)`,
			m.id, "oc-replay-01", string(data), m.ms,
		); err != nil {
			t.Fatal(err)
		}
	}

	verifyLifecycle(t, a, "oc-replay-01", uxp.CLIOpenCode, fixtureCwd, 3)
}

// verifyLifecycle runs the list -> get -> stream chain on one adapter.
func verifyLifecycle(
	t *testing.T,
	a session.SessionAdapter,
	wantID string,
	wantCLI uxp.CLIName,
	wantCwd string,
	wantTurns int,
) {
	t.Helper()

	// 1. ListSessions
	sessions, err := a.ListSessions(wantCwd)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("ListSessions returned 0 sessions")
	}
	found := false
	for _, s := range sessions {
		if s.NativeID == wantID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListSessions did not contain native session %q", wantID)
	}

	// 2. GetSession
	s, err := a.GetSession(wantID)
	if err != nil {
		t.Fatalf("GetSession(%q): %v", wantID, err)
	}
	if s.CLI != wantCLI {
		t.Errorf("CLI = %q, want %q", s.CLI, wantCLI)
	}
	if s.ProjectCwd != wantCwd {
		t.Errorf("ProjectCwd = %q, want %q", s.ProjectCwd, wantCwd)
	}
	// Gemini may not populate TurnCount in GetSession; skip check if 0.
	if s.TurnCount > 0 && s.TurnCount < wantTurns {
		t.Errorf("TurnCount = %d, want >= %d", s.TurnCount, wantTurns)
	}

	// 3. StreamTurns
	ch, err := a.StreamTurns(wantID)
	if err != nil {
		t.Fatalf("StreamTurns(%q): %v", wantID, err)
	}
	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}
	if len(turns) != wantTurns {
		t.Fatalf("StreamTurns returned %d turns, want %d",
			len(turns), wantTurns)
	}
	if turns[0].Content == "" {
		t.Error("first turn has empty content")
	}
}

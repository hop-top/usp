package e2e

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/adapters/opencode"
	"hop.top/usp/index"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

const fixtureCwd = "/tmp/test-project"

// --- fixture writers ---

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

// --- Claude fixtures ---

func setupClaude(t *testing.T, home string) *claude.Adapter {
	t.Helper()
	a := claude.New(claude.WithHomeDir(home))
	key := a.ProjectKey(fixtureCwd)
	dir := filepath.Join(home, ".claude", "projects", key)

	writeJSONL(t, filepath.Join(dir, "claude-sess-01.jsonl"), []map[string]any{
		{
			"uuid": "c1", "type": "user",
			"timestamp": "2026-04-11T09:00:00Z",
			"cwd": fixtureCwd, "sessionId": "claude-sess-01",
			"message": map[string]any{"role": "user", "content": "Hello"},
		},
		{
			"uuid": "c2", "type": "assistant",
			"timestamp": "2026-04-11T09:00:05Z",
			"sessionId": "claude-sess-01",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Hi from Claude"},
				},
			},
		},
	})
	return a
}

// --- Gemini fixtures ---

func setupGemini(t *testing.T, home string) *gemini.Adapter {
	t.Helper()
	alias := "test-project"

	writeJSON(t, filepath.Join(home, ".gemini", "projects.json"),
		map[string]any{
			"projects": map[string]string{fixtureCwd: alias},
		},
	)

	histDir := filepath.Join(home, ".gemini", "history", alias)
	writeJSON(t, filepath.Join(histDir, "gemini-sess-01.json"),
		map[string]any{
			"history": []map[string]any{
				{"role": "user", "content": "Hello Gemini"},
				{"role": "model", "content": "Hi from Gemini"},
			},
		},
	)

	return &gemini.Adapter{HomeDir: home}
}

// --- Codex fixtures ---

func setupCodex(t *testing.T, home string) *codex.Adapter {
	t.Helper()

	sessDir := filepath.Join(
		home, ".codex", "sessions", "2026", "04", "11",
	)

	writeJSONL(t, filepath.Join(sessDir, "codex-sess-01.jsonl"), []map[string]any{
		{
			"timestamp": "2026-04-11T10:00:00Z",
			"type":      "session_meta",
			"payload": map[string]any{
				"id":          "codex-sess-01",
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
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Hello Codex"},
				},
			},
		},
		{
			"timestamp": "2026-04-11T10:00:02Z",
			"type":      "response_item",
			"payload": map[string]any{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Hi from Codex"},
				},
			},
		},
	})

	// Override package-level path funcs for testing.
	restoreSR := codex.SetSessionsRoot(
		filepath.Join(home, ".codex", "sessions"),
	)
	restoreCR := codex.SetCodexRoot(filepath.Join(home, ".codex"))
	t.Cleanup(restoreSR)
	t.Cleanup(restoreCR)

	return &codex.Adapter{}
}

// --- OpenCode fixtures ---

func setupOpenCode(t *testing.T, home string) *opencode.Adapter {
	t.Helper()

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
		`CREATE TABLE session (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			title TEXT,
			directory TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE message (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			data TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (session_id) REFERENCES session(id)
		)`,
		`CREATE TABLE part (
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL,
			data TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (message_id) REFERENCES message(id)
		)`,
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
		"oc-sess-01", projKey, "Test Session",
		fixtureCwd, now, now+5000,
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range []struct {
		id   string
		role string
		text string
		ms   int64
	}{
		{"msg-01", "user", "Hello OpenCode", now},
		{"msg-02", "assistant", "Hi from OpenCode", now + 2000},
	} {
		data, _ := json.Marshal(map[string]string{
			"role": m.role, "content": m.text,
		})
		_, err = db.Exec(
			`INSERT INTO message (id,session_id,data,created_at)
			 VALUES (?,?,?,?)`,
			m.id, "oc-sess-01", string(data), m.ms,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	return a
}

// --- Tests ---

func TestCrossCliSessionList(t *testing.T) {
	home := t.TempDir()

	claudeA := setupClaude(t, home)
	geminiA := setupGemini(t, home)
	codexA := setupCodex(t, home)
	openA := setupOpenCode(t, home)

	adapters := []session.SessionAdapter{claudeA, geminiA, codexA, openA}

	var all []session.Session
	for _, a := range adapters {
		ss, err := a.ListSessions(fixtureCwd)
		if err != nil {
			t.Fatalf("%s ListSessions: %v", a.CLI(), err)
		}
		if len(ss) == 0 {
			t.Fatalf("%s returned 0 sessions", a.CLI())
		}
		all = append(all, ss...)
	}

	if len(all) != 4 {
		t.Fatalf("expected 4 sessions total, got %d", len(all))
	}

	// Verify each CLI is represented.
	seen := map[uxp.CLIName]bool{}
	for _, s := range all {
		seen[s.CLI] = true
	}
	for _, name := range []uxp.CLIName{
		uxp.CLIClaude, uxp.CLIGemini, uxp.CLICodex, uxp.CLIOpenCode,
	} {
		if !seen[name] {
			t.Errorf("missing CLI %q in aggregated sessions", name)
		}
	}
}

func TestCrossCliGetSession(t *testing.T) {
	home := t.TempDir()

	claudeA := setupClaude(t, home)
	geminiA := setupGemini(t, home)
	codexA := setupCodex(t, home)
	openA := setupOpenCode(t, home)

	tests := []struct {
		name     string
		adapter  session.SessionAdapter
		id       string
		wantCLI  uxp.CLIName
		wantCwd  string
		minTurns int // 0 = skip turn check (Gemini lacks turn count in GetSession)
	}{
		{"claude", claudeA, "claude-sess-01",
			uxp.CLIClaude, fixtureCwd, 2},
		{"gemini", geminiA, "gemini-sess-01",
			uxp.CLIGemini, fixtureCwd, 0},
		{"codex", codexA, "codex-sess-01",
			uxp.CLICodex, fixtureCwd, 2},
		{"opencode", openA, "oc-sess-01",
			uxp.CLIOpenCode, fixtureCwd, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := tt.adapter.GetSession(tt.id)
			if err != nil {
				t.Fatalf("GetSession(%q): %v", tt.id, err)
			}
			if s.CLI != tt.wantCLI {
				t.Errorf("CLI = %q, want %q", s.CLI, tt.wantCLI)
			}
			if s.ProjectCwd != tt.wantCwd {
				t.Errorf("ProjectCwd = %q, want %q",
					s.ProjectCwd, tt.wantCwd)
			}
			if s.StartedAt.IsZero() {
				t.Error("StartedAt should not be zero")
			}
			if tt.minTurns > 0 && s.TurnCount < tt.minTurns {
				t.Errorf("TurnCount = %d, want >= %d",
					s.TurnCount, tt.minTurns)
			}
		})
	}
}

func TestProjectIndexScan(t *testing.T) {
	home := t.TempDir()

	claudeA := setupClaude(t, home)
	geminiA := setupGemini(t, home)
	codexA := setupCodex(t, home)
	openA := setupOpenCode(t, home)

	adapters := []session.SessionAdapter{claudeA, geminiA, codexA, openA}

	idxPath := filepath.Join(home, "usp-index.db")
	idx, err := index.Open(idxPath)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	defer idx.Close()

	// Seed the index with the fixture cwd.
	keys := make(map[string]string, len(adapters))
	for _, a := range adapters {
		keys[string(a.CLI())] = a.ProjectKey(fixtureCwd)
	}
	if err := idx.Register(fixtureCwd, keys); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Scan should refresh all entries.
	if err := idx.Scan(adapters); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Lookup should return all 4 CLIs.
	got, err := idx.Lookup(fixtureCwd)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("Lookup returned %d entries, want 4", len(got))
	}
	for _, name := range []uxp.CLIName{
		uxp.CLIClaude, uxp.CLIGemini, uxp.CLICodex, uxp.CLIOpenCode,
	} {
		if _, ok := got[string(name)]; !ok {
			t.Errorf("Lookup missing key for %q", name)
		}
	}
}

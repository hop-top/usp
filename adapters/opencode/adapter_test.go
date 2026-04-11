package opencode

import (
	"database/sql"
	"path/filepath"
	"testing"

	"hop.top/kit/uxp"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

// Compile-time interface satisfaction check.
var _ session.SessionAdapter = (*Adapter)(nil)

// setupFixtureDB creates a temp SQLite database with session/message/part
// tables and inserts test data.
func setupFixtureDB(t *testing.T) (string, *Adapter) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create tables matching OpenCode schema.
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
			session_id TEXT,
			data TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (message_id) REFERENCES message(id)
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("DDL: %v", err)
		}
	}

	// Project hash for /foo/bar.
	a := New(WithDBPath(dbPath))
	projectID := a.ProjectKey("/foo/bar")

	// Insert session.
	_, err = db.Exec(`
		INSERT INTO session (id, project_id, title, directory,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"ses_abc123", projectID, "Test Session", "/foo/bar",
		1712750400000, 1712750460000, // epoch-ms
	)
	if err != nil {
		t.Fatal(err)
	}

	// Insert messages.
	_, err = db.Exec(`
		INSERT INTO message (id, session_id, data, created_at)
		VALUES (?, ?, ?, ?)`,
		"msg_001", "ses_abc123",
		`{"role":"user","content":"Hello OpenCode"}`,
		1712750400000,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO message (id, session_id, data, created_at)
		VALUES (?, ?, ?, ?)`,
		"msg_002", "ses_abc123",
		`{"role":"assistant","content":"Hi there!"}`,
		1712750410000,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Insert parts under msg_002 (by message ID, not session ID).
	_, err = db.Exec(`
		INSERT INTO part (id, message_id, session_id, data, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		"prt_001", "msg_002", "ses_abc123",
		`{"type":"text","text":"Additional context."}`,
		1712750411000,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO part (id, message_id, session_id, data, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		"prt_002", "msg_002", "ses_abc123",
		`{"type":"tool-invocation","toolName":"Read","input":{"path":"/foo/bar/main.go"},"output":"package main"}`,
		1712750412000,
	)
	if err != nil {
		t.Fatal(err)
	}

	return dir, a
}

func TestCLI(t *testing.T) {
	a := New()
	if a.CLI() != uxp.CLIOpenCode {
		t.Errorf("CLI() = %q, want %q", a.CLI(), uxp.CLIOpenCode)
	}
}

func TestProjectKey(t *testing.T) {
	a := New()
	tests := []struct {
		cwd  string
		want string
	}{
		{
			cwd:  "/foo/bar",
			want: "a82cce35fd860de6f63f97e6c482dc6a14d002e8",
		},
		{
			cwd:  "/Users/jadb/Repositories/oss-xray-codes",
			want: "e27d85e3195bbdd12dc0e367453dafeefad36862",
		},
	}
	for _, tt := range tests {
		t.Run(tt.cwd, func(t *testing.T) {
			got := a.ProjectKey(tt.cwd)
			if got != tt.want {
				t.Errorf("ProjectKey(%q) = %q, want %q",
					tt.cwd, got, tt.want)
			}
			if len(got) != 40 {
				t.Errorf("expected 40-char hex, got %d chars", len(got))
			}
		})
	}
}

func TestCapabilities(t *testing.T) {
	a := New()
	caps := a.Capabilities()

	// native
	if !caps.Supports("session.list") {
		t.Error("expected native support for session.list")
	}
	if !caps.Supports("event.sourcing") {
		t.Error("expected native support for event.sourcing")
	}
	// workaround
	if !caps.Supports("session.search") {
		t.Error("expected workaround support for session.search")
	}
	// missing
	if caps.Supports("project.stable.id") {
		t.Error("expected missing support for project.stable.id")
	}
	// unknown
	if caps.Supports("nonexistent.dimension") {
		t.Error("expected no support for unknown dimension")
	}

	cov := caps.Coverage()
	if len(cov) != 19 {
		t.Errorf("Coverage() returned %d dimensions, want 19", len(cov))
	}
}

func TestListSessions(t *testing.T) {
	_, a := setupFixtureDB(t)

	sessions, err := a.ListSessions("/foo/bar")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}

	s := sessions[0]
	if s.ID != "ses_abc123" {
		t.Errorf("ID = %q, want %q", s.ID, "ses_abc123")
	}
	if s.CLI != uxp.CLIOpenCode {
		t.Errorf("CLI = %q, want %q", s.CLI, uxp.CLIOpenCode)
	}
	if s.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", s.TurnCount)
	}
	if s.ProjectCwd != "/foo/bar" {
		t.Errorf("ProjectCwd = %q, want /foo/bar", s.ProjectCwd)
	}
	if s.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
	if s.EndedAt == nil {
		t.Error("EndedAt should not be nil")
	}
	if s.Metadata["title"] != "Test Session" {
		t.Errorf("title = %v, want %q", s.Metadata["title"], "Test Session")
	}
}

func TestListSessionsMissingDB(t *testing.T) {
	a := New(WithDBPath("/nonexistent/opencode.db"))
	sessions, err := a.ListSessions("/foo/bar")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected empty sessions, got %d", len(sessions))
	}
}

func TestGetSession(t *testing.T) {
	_, a := setupFixtureDB(t)

	s, err := a.GetSession("ses_abc123")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.ID != "ses_abc123" {
		t.Errorf("ID = %q, want %q", s.ID, "ses_abc123")
	}
	if s.ProjectCwd != "/foo/bar" {
		t.Errorf("ProjectCwd = %q, want /foo/bar", s.ProjectCwd)
	}
	if s.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", s.TurnCount)
	}
	if s.Metadata["title"] != "Test Session" {
		t.Errorf("title = %v, want %q", s.Metadata["title"], "Test Session")
	}
}

func TestGetSessionNotFound(t *testing.T) {
	_, a := setupFixtureDB(t)

	_, err := a.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestStreamTurns(t *testing.T) {
	_, a := setupFixtureDB(t)

	ch, err := a.StreamTurns("ses_abc123")
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}

	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}

	if len(turns) != 2 {
		t.Fatalf("got %d turns, want 2", len(turns))
	}

	// First turn: user message.
	if turns[0].Role != session.RoleUser {
		t.Errorf("turn[0].Role = %q, want user", turns[0].Role)
	}
	if turns[0].Content != "Hello OpenCode" {
		t.Errorf("turn[0].Content = %q, want %q",
			turns[0].Content, "Hello OpenCode")
	}

	// Second turn: assistant with parts.
	if turns[1].Role != session.RoleAssistant {
		t.Errorf("turn[1].Role = %q, want assistant", turns[1].Role)
	}
	// Content from message + text part.
	want := "Hi there!\nAdditional context."
	if turns[1].Content != want {
		t.Errorf("turn[1].Content = %q, want %q", turns[1].Content, want)
	}

	// Tool call from part (keyed by message ID).
	if len(turns[1].ToolCalls) != 1 {
		t.Fatalf("turn[1].ToolCalls len = %d, want 1",
			len(turns[1].ToolCalls))
	}
	if turns[1].ToolCalls[0].Name != "Read" {
		t.Errorf("tool call name = %q, want Read",
			turns[1].ToolCalls[0].Name)
	}
	if turns[1].ToolCalls[0].Output != "package main" {
		t.Errorf("tool call output = %q, want %q",
			turns[1].ToolCalls[0].Output, "package main")
	}
}

func TestStreamTurnsOrdering(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, ddl := range []string{
		`CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT,
			title TEXT, directory TEXT, created_at INTEGER,
			updated_at INTEGER)`,
		`CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT,
			data TEXT, created_at INTEGER)`,
		`CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT,
			session_id TEXT, data TEXT, created_at INTEGER)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}

	a := New(WithDBPath(dbPath))
	projectID := a.ProjectKey("/test")

	db.Exec(`INSERT INTO session VALUES (?, ?, ?, ?, ?, ?)`,
		"ses_order", projectID, "", "/test", 1000, 3000)

	// Insert messages out of order to verify ORDER BY.
	db.Exec(`INSERT INTO message VALUES (?, ?, ?, ?)`,
		"msg_b", "ses_order",
		`{"role":"assistant","content":"second"}`, 2000)
	db.Exec(`INSERT INTO message VALUES (?, ?, ?, ?)`,
		"msg_a", "ses_order",
		`{"role":"user","content":"first"}`, 1000)
	db.Exec(`INSERT INTO message VALUES (?, ?, ?, ?)`,
		"msg_c", "ses_order",
		`{"role":"user","content":"third"}`, 3000)
	db.Close()

	ch, err := a.StreamTurns("ses_order")
	if err != nil {
		t.Fatal(err)
	}

	var contents []string
	for turn := range ch {
		contents = append(contents, turn.Content)
	}

	expected := []string{"first", "second", "third"}
	if len(contents) != len(expected) {
		t.Fatalf("got %d turns, want %d", len(contents), len(expected))
	}
	for i, want := range expected {
		if contents[i] != want {
			t.Errorf("turn[%d].Content = %q, want %q",
				i, contents[i], want)
		}
	}
}

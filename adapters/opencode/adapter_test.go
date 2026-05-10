package opencode

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

// Compile-time interface satisfaction checks.
var _ session.SessionAdapter = (*Adapter)(nil)
var _ session.ResumeAdapter = (*Adapter)(nil)

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
	if s.NativeID != "ses_abc123" {
		t.Errorf("NativeID = %q, want %q", s.NativeID, "ses_abc123")
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
	if s.NativeID != "ses_abc123" {
		t.Errorf("NativeID = %q, want %q", s.NativeID, "ses_abc123")
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

func TestResumeCmd(t *testing.T) {
	a := New()
	cmd := a.ResumeCmd("ses_abc123")
	// opencode's resume is `opencode run --session <id>`; the `run`
	// subcommand is required. The previous in-tree string omitted
	// it (likely a latent bug); kit's invocation facade emits the
	// correct form.
	want := []string{"opencode", "run", "--session", "ses_abc123"}
	if len(cmd) != len(want) {
		t.Fatalf("ResumeCmd len = %d, want %d", len(cmd), len(want))
	}
	for i := range want {
		if cmd[i] != want[i] {
			t.Errorf("ResumeCmd[%d] = %q, want %q", i, cmd[i], want[i])
		}
	}
}

func TestResumeAdapterInterface(t *testing.T) {
	// Compile-time check is via var _ above; this confirms at runtime.
	var ra session.ResumeAdapter = New()
	if ra.CLI() != uxp.CLIOpenCode {
		t.Errorf("CLI() = %q, want %q", ra.CLI(), uxp.CLIOpenCode)
	}
}

func TestGetSession_AssistantModelFromMessage(t *testing.T) {
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
	projectID := a.ProjectKey("/p")

	if _, err := db.Exec(
		`INSERT INTO session VALUES (?, ?, ?, ?, ?, ?)`,
		"ses_model", projectID, "T", "/p", 1000, 2000,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO message VALUES (?, ?, ?, ?)`,
		"m1", "ses_model", `{"role":"user","content":"hi"}`, 1000,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO message VALUES (?, ?, ?, ?)`,
		"m2", "ses_model",
		`{"role":"assistant","content":"hello","modelID":"glm-4.7"}`,
		2000,
	); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := a.GetSession("ses_model")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got := s.Metadata["assistant.model"]; got != "glm-4.7" {
		t.Errorf("assistant.model = %v, want glm-4.7", got)
	}
}

func TestGetSession_AssistantModelAbsent(t *testing.T) {
	// Existing fixture has no modelID on message rows.
	_, a := setupFixtureDB(t)
	s, err := a.GetSession("ses_abc123")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if _, ok := s.Metadata["assistant.model"]; ok {
		t.Errorf("assistant.model should be absent, got %v",
			s.Metadata["assistant.model"])
	}
}

func TestInjectSession(t *testing.T) {
	_, a := setupFixtureDB(t)

	ts := time.Date(2024, 4, 10, 12, 0, 0, 0, time.UTC)
	turns := []session.Turn{
		{
			Role:      session.RoleUser,
			Content:   "Implement the feature",
			Timestamp: ts,
		},
		{
			Role:      session.RoleAssistant,
			Content:   "Sure, working on it.",
			Timestamp: ts.Add(10 * time.Second),
			ToolCalls: []session.ToolCall{
				{
					Name:   "Edit",
					Input:  `{"file":"main.go"}`,
					Output: "done",
				},
			},
		},
	}

	sesID, err := a.InjectSession("/foo/bar", turns)
	if err != nil {
		t.Fatalf("InjectSession: %v", err)
	}

	// Verify ID format.
	if !strings.HasPrefix(sesID, "ses_") {
		t.Errorf("session ID %q missing ses_ prefix", sesID)
	}
	if len(sesID) != 30 { // "ses_" (4) + 26 chars
		t.Errorf("session ID len = %d, want 30", len(sesID))
	}

	// Read back via GetSession.
	s, err := a.GetSession(sesID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", s.TurnCount)
	}
	if s.Metadata["title"] != "USP resumed session" {
		t.Errorf("title = %v, want %q", s.Metadata["title"],
			"USP resumed session")
	}
	if s.ProjectCwd != "/foo/bar" {
		t.Errorf("ProjectCwd = %q, want /foo/bar", s.ProjectCwd)
	}

	// Read back via StreamTurns.
	ch, err := a.StreamTurns(sesID)
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}

	var got []session.Turn
	for turn := range ch {
		got = append(got, turn)
	}
	if len(got) != 2 {
		t.Fatalf("got %d turns, want 2", len(got))
	}

	if got[0].Role != session.RoleUser {
		t.Errorf("turn[0].Role = %q, want user", got[0].Role)
	}
	if got[0].Content != "Implement the feature" {
		t.Errorf("turn[0].Content = %q", got[0].Content)
	}

	if got[1].Role != session.RoleAssistant {
		t.Errorf("turn[1].Role = %q, want assistant", got[1].Role)
	}
	// Content comes from message data + text part.
	if !strings.Contains(got[1].Content, "Sure, working on it.") {
		t.Errorf("turn[1].Content missing assistant text: %q",
			got[1].Content)
	}

	if len(got[1].ToolCalls) != 1 {
		t.Fatalf("turn[1].ToolCalls len = %d, want 1",
			len(got[1].ToolCalls))
	}
	if got[1].ToolCalls[0].Name != "Edit" {
		t.Errorf("tool call name = %q, want Edit",
			got[1].ToolCalls[0].Name)
	}
	if got[1].ToolCalls[0].Output != "done" {
		t.Errorf("tool call output = %q, want done",
			got[1].ToolCalls[0].Output)
	}
}

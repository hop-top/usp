package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// --- interface satisfaction ---

func TestAdapterSatisfiesInterface(t *testing.T) {
	var _ session.SessionAdapter = (*Adapter)(nil)
}

// --- helpers ---

func writeJSONL(t *testing.T, path string, lines ...any) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, l := range lines {
		if err := enc.Encode(l); err != nil {
			t.Fatal(err)
		}
	}
}

func makeMeta(id, cwd string) map[string]any {
	return map[string]any{
		"timestamp": "2026-04-09T17:34:10.498Z",
		"type":      "session_meta",
		"payload": map[string]any{
			"id":          id,
			"timestamp":   "2026-04-09T17:32:48.478Z",
			"cwd":         cwd,
			"originator":  "codex_cli_rs",
			"cli_version": "0.119.0-alpha.11",
			"source":      "cli",
		},
	}
}

func makeResponseItem(role, text string) map[string]any {
	return map[string]any{
		"timestamp": "2026-04-09T17:35:00.000Z",
		"type":      "response_item",
		"payload": map[string]any{
			"type": "message",
			"role": role,
			"content": []map[string]any{
				{"type": "input_text", "text": text},
			},
		},
	}
}

// makeOldFormatItem simulates older Codex schema (pre-response_item).
func makeOldFormatItem(role, text string) map[string]any {
	return map[string]any{
		"timestamp": "2025-10-04T03:01:00.000Z",
		"type":      "event",
		"payload": map[string]any{
			"role": role,
			"content": []map[string]any{
				{"type": "input_text", "text": text},
			},
		},
	}
}

// setupSessionTree creates a date-partitioned session dir tree in tmp.
// Returns the sessions root dir.
func setupSessionTree(t *testing.T, cwd string) string {
	t.Helper()
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".codex", "sessions")

	// Create a session matching cwd.
	dayDir := filepath.Join(root, "2026", "04", "09")
	os.MkdirAll(dayDir, 0o755)

	id := "019cb730-aaaa-7000-bbbb-ccccddddeeee"
	fname := "rollout-2026-04-09T17-34-10-" + id + ".jsonl"
	writeJSONL(t, filepath.Join(dayDir, fname),
		makeMeta(id, cwd),
		makeResponseItem("user", "hello codex"),
		makeResponseItem("assistant", "hi there"),
	)

	// Create a session with different cwd.
	id2 := "019cb730-ffff-7000-aaaa-111122223333"
	fname2 := "rollout-2026-04-09T17-40-00-" + id2 + ".jsonl"
	writeJSONL(t, filepath.Join(dayDir, fname2),
		makeMeta(id2, "/other/project"),
		makeResponseItem("user", "other project prompt"),
	)

	return root
}

// --- tests ---

func TestListSessionsWalk(t *testing.T) {
	cwd := "/Users/test/myproject"
	root := setupSessionTree(t, cwd)

	a := &Adapter{}

	// Override sessionsRoot for testing.
	origRoot := sessionsRootFn
	sessionsRootFn = func() (string, error) { return root, nil }
	defer func() { sessionsRootFn = origRoot }()

	sessions, err := a.listFromWalk(root, cwd)
	if err != nil {
		t.Fatal(err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.CLI != uxp.CLICodex {
		t.Errorf("cli = %q, want %q", s.CLI, uxp.CLICodex)
	}
	if s.ProjectCwd != cwd {
		t.Errorf("cwd = %q, want %q", s.ProjectCwd, cwd)
	}
	if s.TurnCount != 2 {
		t.Errorf("turn_count = %d, want 2", s.TurnCount)
	}
	if s.NativeID != "019cb730-aaaa-7000-bbbb-ccccddddeeee" {
		t.Errorf("native_id = %q, want uuid", s.NativeID)
	}
}

func TestListSessionsIndex(t *testing.T) {
	cwd := "/Users/test/myproject"
	root := setupSessionTree(t, cwd)

	// Create session_index.jsonl in codex root (parent of sessions/).
	codexDir := filepath.Dir(root)
	indexPath := filepath.Join(codexDir, "session_index.jsonl")

	id := "019cb730-aaaa-7000-bbbb-ccccddddeeee"
	writeJSONL(t, indexPath,
		indexEntry{ID: id, ThreadName: "Plan refactor", UpdatedAt: "2026-04-09T18:00:00Z"},
		indexEntry{ID: "019cb730-ffff-7000-aaaa-111122223333", ThreadName: "Other", UpdatedAt: "2026-04-09T18:01:00Z"},
	)

	a := &Adapter{}

	origRoot := codexRootFn
	codexRootFn = func() (string, error) { return codexDir, nil }
	defer func() { codexRootFn = origRoot }()

	sessions, err := a.listFromIndex(root, cwd)
	if err != nil {
		t.Fatal(err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.Metadata["thread_name"] != "Plan refactor" {
		t.Errorf("thread_name = %v, want %q", s.Metadata["thread_name"], "Plan refactor")
	}
}

func TestListSessionsFallback(t *testing.T) {
	cwd := "/Users/test/myproject"
	root := setupSessionTree(t, cwd)
	codexDir := filepath.Dir(root)

	a := &Adapter{}

	origSR := sessionsRootFn
	sessionsRootFn = func() (string, error) { return root, nil }
	defer func() { sessionsRootFn = origSR }()

	origCR := codexRootFn
	codexRootFn = func() (string, error) { return codexDir, nil }
	defer func() { codexRootFn = origCR }()

	// No index file exists -> should fall back to walk.
	sessions, err := a.ListSessions(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session via fallback, got %d", len(sessions))
	}
}

func TestGetSession(t *testing.T) {
	cwd := "/Users/test/myproject"
	root := setupSessionTree(t, cwd)

	origRoot := sessionsRootFn
	sessionsRootFn = func() (string, error) { return root, nil }
	defer func() { sessionsRootFn = origRoot }()

	a := &Adapter{}
	s, err := a.GetSession("019cb730-aaaa-7000-bbbb-ccccddddeeee")
	if err != nil {
		t.Fatal(err)
	}
	if s.ProjectCwd != cwd {
		t.Errorf("cwd = %q, want %q", s.ProjectCwd, cwd)
	}
}

func TestStreamTurns(t *testing.T) {
	cwd := "/Users/test/myproject"
	root := setupSessionTree(t, cwd)

	origRoot := sessionsRootFn
	sessionsRootFn = func() (string, error) { return root, nil }
	defer func() { sessionsRootFn = origRoot }()

	a := &Adapter{}
	ch, err := a.StreamTurns("019cb730-aaaa-7000-bbbb-ccccddddeeee")
	if err != nil {
		t.Fatal(err)
	}

	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}

	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if turns[0].Role != session.RoleUser {
		t.Errorf("turn[0].role = %q, want user", turns[0].Role)
	}
	if turns[0].Content != "hello codex" {
		t.Errorf("turn[0].content = %q, want %q", turns[0].Content, "hello codex")
	}
	if turns[1].Role != session.RoleAssistant {
		t.Errorf("turn[1].role = %q, want assistant", turns[1].Role)
	}
}

func TestStreamTurnsOldFormat(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".codex", "sessions")
	dayDir := filepath.Join(root, "2025", "10", "04")
	os.MkdirAll(dayDir, 0o755)

	id := "019aa000-1111-7000-2222-333344445555"
	fname := "rollout-2025-10-04T03-00-25-" + id + ".jsonl"

	// Old-format meta (instructions: null, no base_instructions).
	oldMeta := map[string]any{
		"timestamp": "2025-10-04T03:00:25.000Z",
		"type":      "session_meta",
		"payload": map[string]any{
			"id":           id,
			"timestamp":    "2025-10-04T03:00:25.000Z",
			"cwd":          "/old/project",
			"originator":   "codex_cli_rs",
			"cli_version":  "0.44.0",
			"instructions": nil,
		},
	}

	writeJSONL(t, filepath.Join(dayDir, fname),
		oldMeta,
		makeOldFormatItem("user", "old prompt"),
		makeOldFormatItem("assistant", "old response"),
	)

	origRoot := sessionsRootFn
	sessionsRootFn = func() (string, error) { return root, nil }
	defer func() { sessionsRootFn = origRoot }()

	a := &Adapter{}
	ch, err := a.StreamTurns(id)
	if err != nil {
		t.Fatal(err)
	}

	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}

	if len(turns) != 2 {
		t.Fatalf("expected 2 turns from old format, got %d", len(turns))
	}
	if turns[0].Content != "old prompt" {
		t.Errorf("turn[0].content = %q, want %q", turns[0].Content, "old prompt")
	}
}

func TestProjectKey(t *testing.T) {
	a := &Adapter{}
	cwd := "/Users/test/myproject"
	if got := a.ProjectKey(cwd); got != cwd {
		t.Errorf("ProjectKey = %q, want %q", got, cwd)
	}
}

func TestCapabilities(t *testing.T) {
	a := &Adapter{}
	caps := a.Capabilities()

	if !caps.Supports("session_store") {
		t.Error("expected session_store to be supported")
	}
	if !caps.Supports("project_grouping") {
		t.Error("expected project_grouping workaround to be supported")
	}
	if caps.Supports("session_branching") {
		t.Error("expected session_branching to be missing")
	}

	coverage := caps.Coverage()
	if len(coverage) != 20 {
		t.Errorf("coverage has %d dims, want 20", len(coverage))
	}
}

func TestResumeAdapterInterface(t *testing.T) {
	var _ session.ResumeAdapter = (*Adapter)(nil)
}

func TestInjectSession(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".codex", "sessions")
	restore := SetSessionsRoot(root)
	defer restore()

	cwd := "/Users/test/injected"
	turns := []session.Turn{
		{Role: session.RoleUser, Content: "hello from claude", Timestamp: time.Now()},
		{Role: session.RoleAssistant, Content: "hi back", Timestamp: time.Now(),
			ToolCalls: []session.ToolCall{{Name: "Bash", Output: "ok"}}},
	}

	a := &Adapter{}
	id, err := a.InjectSession(cwd, turns)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty session ID")
	}
	// UUID format check
	if len(id) != 36 || id[8] != '-' || id[13] != '-' {
		t.Errorf("id %q not UUID-shaped", id)
	}

	// Read back via GetSession
	s, err := a.GetSession(id)
	if err != nil {
		t.Fatalf("GetSession(%s): %v", id, err)
	}
	if s.ProjectCwd != cwd {
		t.Errorf("cwd = %q, want %q", s.ProjectCwd, cwd)
	}
	if s.TurnCount != 2 {
		t.Errorf("turn_count = %d, want 2", s.TurnCount)
	}
	if s.Metadata["originator"] != "usp" {
		t.Errorf("originator = %v, want usp", s.Metadata["originator"])
	}

	// Verify turns via StreamTurns
	ch, err := a.StreamTurns(id)
	if err != nil {
		t.Fatal(err)
	}
	var got []session.Turn
	for turn := range ch {
		got = append(got, turn)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(got))
	}
	if got[0].Role != session.RoleUser || got[0].Content != "hello from claude" {
		t.Errorf("turn[0] = %+v", got[0])
	}
	if !strings.Contains(got[1].Content, "[Tool: Bash") {
		t.Errorf("turn[1].content = %q, want tool summary", got[1].Content)
	}
}

func TestResumeCmd(t *testing.T) {
	a := &Adapter{}
	cmd := a.ResumeCmd("abc-123")
	want := []string{"codex", "resume", "abc-123"}
	if len(cmd) != len(want) {
		t.Fatalf("cmd = %v, want %v", cmd, want)
	}
	for i := range want {
		if cmd[i] != want[i] {
			t.Errorf("cmd[%d] = %q, want %q", i, cmd[i], want[i])
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	ts := "2026-04-09T17:32:48.478Z"
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Year() != 2026 || parsed.Month() != 4 {
		t.Errorf("parsed = %v, unexpected", parsed)
	}
}

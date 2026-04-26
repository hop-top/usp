package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// Compile-time interface satisfaction checks.
var _ session.SessionAdapter = (*Adapter)(nil)
var _ session.ResumeAdapter = (*Adapter)(nil)

func TestProjectKey(t *testing.T) {
	a := New()
	tests := []struct {
		cwd  string
		want string
	}{
		{
			cwd:  "/Users/jadb/.w/ideacrafterslabs/uhp/hops/main",
			want: "-Users-jadb--w-ideacrafterslabs-uhp-hops-main",
		},
		{
			cwd:  "/Users/jadb/.config/something",
			want: "-Users-jadb--config-something",
		},
		{
			cwd:  "/foo/bar",
			want: "-foo-bar",
		},
		{
			cwd:  "/foo/.bar/baz",
			want: "-foo--bar-baz",
		},
		{
			cwd:  "/Users/jadb/.agents",
			want: "-Users-jadb--agents",
		},
	}
	for _, tt := range tests {
		t.Run(tt.cwd, func(t *testing.T) {
			got := a.ProjectKey(tt.cwd)
			if got != tt.want {
				t.Errorf("ProjectKey(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}

func TestCLI(t *testing.T) {
	a := New()
	if a.CLI() != uxp.CLIClaude {
		t.Errorf("CLI() = %q, want %q", a.CLI(), uxp.CLIClaude)
	}
}

func TestCapabilities(t *testing.T) {
	a := New()
	caps := a.Capabilities()

	if !caps.Supports("session.list") {
		t.Error("expected native support for session.list")
	}
	if !caps.Supports("session.search") {
		t.Error("expected workaround support for session.search")
	}
	if caps.Supports("session.cross.project") {
		t.Error("expected missing support for session.cross.project")
	}
	if caps.Supports("nonexistent.dimension") {
		t.Error("expected no support for unknown dimension")
	}

	cov := caps.Coverage()
	if len(cov) != 20 {
		t.Errorf("Coverage() returned %d dimensions, want 20", len(cov))
	}
}

func writeFixtureJSONL(t *testing.T, dir, filename string, events []map[string]any) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	f, err := os.Create(p)
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
	return p
}

func setupFixtures(t *testing.T) (string, *Adapter) {
	t.Helper()
	home := t.TempDir()

	// Project key for /foo/bar => -foo-bar
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	events := []map[string]any{
		{
			"uuid":      "evt-001",
			"type":      "user",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd":       "/foo/bar",
			"gitBranch": "main",
			"sessionId": "sess-abc",
			"message": map[string]any{
				"role":    "user",
				"content": "Hello, Claude!",
			},
		},
		{
			"uuid":       "evt-002",
			"parentUuid": "evt-001",
			"type":       "assistant",
			"timestamp":  "2026-04-10T10:00:05.000Z",
			"sessionId":  "sess-abc",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Hi there!"},
					{
						"type":  "tool_use",
						"name":  "Read",
						"input": map[string]any{"path": "/foo/bar/main.go"},
					},
				},
			},
		},
		{
			"uuid":       "evt-003",
			"parentUuid": "evt-002",
			"type":       "tool_result",
			"timestamp":  "2026-04-10T10:00:06.000Z",
		},
		{
			"uuid":       "evt-004",
			"parentUuid": "evt-003",
			"type":       "user",
			"timestamp":  "2026-04-10T10:01:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "Thanks!",
			},
		},
	}

	writeFixtureJSONL(t, projDir, "sess-abc.jsonl", events)

	a := New(WithHomeDir(home))
	return home, a
}

func TestListSessions(t *testing.T) {
	_, a := setupFixtures(t)

	sessions, err := a.ListSessions("/foo/bar")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}

	s := sessions[0]
	if s.NativeID != "sess-abc" {
		t.Errorf("NativeID = %q, want %q", s.NativeID, "sess-abc")
	}
	if s.ID == "" {
		t.Error("ID (TypeID) should be populated")
	}
	if s.CLI != uxp.CLIClaude {
		t.Errorf("CLI = %q, want %q", s.CLI, uxp.CLIClaude)
	}
	if s.TurnCount != 3 {
		t.Errorf("TurnCount = %d, want 3", s.TurnCount)
	}
	if s.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
	if s.EndedAt == nil {
		t.Error("EndedAt should not be nil")
	}
	if s.Metadata["gitBranch"] != "main" {
		t.Errorf("gitBranch = %v, want %q", s.Metadata["gitBranch"], "main")
	}
}

func TestGetSession(t *testing.T) {
	_, a := setupFixtures(t)

	s, err := a.GetSession("sess-abc")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.NativeID != "sess-abc" {
		t.Errorf("NativeID = %q, want %q", s.NativeID, "sess-abc")
	}
	if s.ProjectCwd != "/foo/bar" {
		t.Errorf("ProjectCwd = %q, want %q", s.ProjectCwd, "/foo/bar")
	}
	if s.TurnCount != 3 {
		t.Errorf("TurnCount = %d, want 3", s.TurnCount)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	_, a := setupFixtures(t)

	_, err := a.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestStreamTurns(t *testing.T) {
	_, a := setupFixtures(t)

	ch, err := a.StreamTurns("sess-abc")
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}

	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}

	// user + assistant + user = 3 (tool_result is skipped)
	if len(turns) != 3 {
		t.Fatalf("got %d turns, want 3", len(turns))
	}

	if turns[0].Role != session.RoleUser {
		t.Errorf("turn[0].Role = %q, want %q", turns[0].Role, session.RoleUser)
	}
	if turns[0].Content != "Hello, Claude!" {
		t.Errorf("turn[0].Content = %q, want %q", turns[0].Content, "Hello, Claude!")
	}

	if turns[1].Role != session.RoleAssistant {
		t.Errorf("turn[1].Role = %q, want %q", turns[1].Role, session.RoleAssistant)
	}
	if turns[1].Content != "Hi there!" {
		t.Errorf("turn[1].Content = %q, want %q", turns[1].Content, "Hi there!")
	}
	if len(turns[1].ToolCalls) != 1 {
		t.Fatalf("turn[1].ToolCalls len = %d, want 1", len(turns[1].ToolCalls))
	}
	if turns[1].ToolCalls[0].Name != "Read" {
		t.Errorf("tool call name = %q, want %q", turns[1].ToolCalls[0].Name, "Read")
	}

	if turns[2].Role != session.RoleUser {
		t.Errorf("turn[2].Role = %q, want %q", turns[2].Role, session.RoleUser)
	}
	if turns[2].Content != "Thanks!" {
		t.Errorf("turn[2].Content = %q, want %q", turns[2].Content, "Thanks!")
	}
}

func TestProjectKeyDotReplacement(t *testing.T) {
	a := New()

	// .w in path produces double-dash (dot becomes dash)
	got := a.ProjectKey("/Users/jadb/.w/ideacrafterslabs/uhp/hops/main")
	want := "-Users-jadb--w-ideacrafterslabs-uhp-hops-main"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// .hidden dirs get dot replaced with dash
	got = a.ProjectKey("/home/user/.local/share")
	want = "-home-user--local-share"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// matches real Claude Code dir observed on disk
	got = a.ProjectKey("/Users/jadb/.agents")
	want = "-Users-jadb--agents"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInjectSession(t *testing.T) {
	home := t.TempDir()
	a := New(WithHomeDir(home))

	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	turns := []session.Turn{
		{Role: session.RoleUser, Content: "Hello", Timestamp: now},
		{
			Role: session.RoleAssistant, Content: "Hi!",
			Timestamp: now.Add(time.Second),
			ToolCalls: []session.ToolCall{
				{Name: "Read", Input: "/tmp/f.txt", Output: "contents"},
			},
		},
		{Role: session.RoleUser, Content: "Thanks", Timestamp: now.Add(2 * time.Second)},
	}

	uuid, err := a.InjectSession("/foo/bar", turns)
	if err != nil {
		t.Fatalf("InjectSession: %v", err)
	}
	if uuid == "" {
		t.Fatal("expected non-empty UUID")
	}

	// Verify file exists.
	path := filepath.Join(home, ".claude", "projects", "-foo-bar", uuid+".jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("JSONL file missing: %v", err)
	}

	// Round-trip: read back via GetSession.
	s, err := a.GetSession(uuid)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.TurnCount != 3 {
		t.Errorf("TurnCount = %d, want 3", s.TurnCount)
	}

	// Round-trip: stream turns back.
	ch, err := a.StreamTurns(uuid)
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}
	var got []session.Turn
	for turn := range ch {
		got = append(got, turn)
	}
	if len(got) != 3 {
		t.Fatalf("got %d turns, want 3", len(got))
	}
	if got[0].Content != "Hello" {
		t.Errorf("turn[0].Content = %q, want %q", got[0].Content, "Hello")
	}
	if got[2].Content != "Thanks" {
		t.Errorf("turn[2].Content = %q, want %q", got[2].Content, "Thanks")
	}
	// Assistant turn includes tool call summary.
	if got[1].Role != session.RoleAssistant {
		t.Errorf("turn[1].Role = %q, want assistant", got[1].Role)
	}
}

func TestExtractContent_ToolResultString(t *testing.T) {
	raw := json.RawMessage(`[{"type":"tool_result","tool_use_id":"t1","content":"file contents here"}]`)
	got := extractContent(raw)
	if got != "file contents here" {
		t.Errorf("extractContent tool_result string = %q, want %q", got, "file contents here")
	}
}

func TestExtractContent_ToolResultArray(t *testing.T) {
	raw := json.RawMessage(`[{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"line one"},{"type":"text","text":"line two"}]}]`)
	got := extractContent(raw)
	if got != "line one\nline two" {
		t.Errorf("extractContent tool_result array = %q, want %q", got, "line one\nline two")
	}
}

func TestExtractContent_MixedTextAndToolResult(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"preamble"},{"type":"tool_result","tool_use_id":"t1","content":"result data"}]`)
	got := extractContent(raw)
	if got != "preamble\nresult data" {
		t.Errorf("extractContent mixed = %q, want %q", got, "preamble\nresult data")
	}
}

func TestStreamTurns_ToolResultUserTurn(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	events := []map[string]any{
		{
			"uuid": "e1", "type": "user",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd": "/foo/bar", "sessionId": "sess-tr",
			"message": map[string]any{
				"role":    "user",
				"content": "read main.go",
			},
		},
		{
			"uuid": "e2", "type": "assistant",
			"timestamp": "2026-04-10T10:00:05.000Z",
			"sessionId": "sess-tr",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Reading file..."},
					{"type": "tool_use", "name": "Read", "input": map[string]any{"path": "/foo/bar/main.go"}},
				},
			},
		},
		{
			"uuid": "e3", "type": "user",
			"timestamp": "2026-04-10T10:00:06.000Z",
			"sessionId": "sess-tr",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "tu1", "content": "package main\nfunc main() {}"},
				},
			},
		},
	}

	writeFixtureJSONL(t, projDir, "sess-tr.jsonl", events)
	a := New(WithHomeDir(home))

	ch, err := a.StreamTurns("sess-tr")
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}
	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}
	if len(turns) != 3 {
		t.Fatalf("got %d turns, want 3", len(turns))
	}
	// tool_result user turn must preserve content
	if turns[2].Content == "" {
		t.Error("tool_result user turn has empty content")
	}
	if turns[2].Content != "package main\nfunc main() {}" {
		t.Errorf("turn[2].Content = %q, want tool result content", turns[2].Content)
	}
}

func TestStreamTurns_ToolResultInlined(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	events := []map[string]any{
		{
			"uuid": "e1", "type": "user",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd": "/foo/bar", "sessionId": "sess-inline",
			"message": map[string]any{
				"role":    "user",
				"content": "read main.go",
			},
		},
		{
			"uuid": "e2", "type": "assistant",
			"timestamp": "2026-04-10T10:00:05.000Z",
			"sessionId": "sess-inline",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Reading file..."},
					{"type": "tool_use", "id": "toolu_abc123", "name": "Read",
						"input": map[string]any{"path": "/foo/bar/main.go"}},
				},
			},
		},
		{
			"uuid": "e3", "type": "user",
			"timestamp": "2026-04-10T10:00:06.000Z",
			"sessionId": "sess-inline",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "toolu_abc123",
						"content": "package main\nfunc main() {}"},
				},
			},
		},
		{
			"uuid": "e4", "type": "user",
			"timestamp": "2026-04-10T10:00:10.000Z",
			"sessionId": "sess-inline",
			"message": map[string]any{
				"role":    "user",
				"content": "looks good",
			},
		},
	}

	writeFixtureJSONL(t, projDir, "sess-inline.jsonl", events)
	a := New(WithHomeDir(home))

	ch, err := a.StreamTurns("sess-inline")
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}
	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}

	// user + assistant + tool_result user + plain user = 4
	if len(turns) != 4 {
		t.Fatalf("got %d turns, want 4", len(turns))
	}

	// assistant turn must have tool call with Output resolved
	if len(turns[1].ToolCalls) != 1 {
		t.Fatalf("turn[1].ToolCalls len = %d, want 1", len(turns[1].ToolCalls))
	}
	tc := turns[1].ToolCalls[0]
	if tc.Name != "Read" {
		t.Errorf("tool call name = %q, want Read", tc.Name)
	}
	const wantOutput = "package main\nfunc main() {}"
	if tc.Output != wantOutput {
		t.Errorf("tool call Output = %q, want %q", tc.Output, wantOutput)
	}

	// tool_result user turn still emitted with its content
	if turns[2].Role != session.RoleUser {
		t.Errorf("turn[2].Role = %q, want user", turns[2].Role)
	}
	if turns[2].Content != wantOutput {
		t.Errorf("turn[2].Content = %q, want %q", turns[2].Content, wantOutput)
	}
}

func TestStreamTurns_MultipleToolCallsInlined(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	events := []map[string]any{
		{
			"uuid": "e1", "type": "assistant",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd": "/foo/bar", "sessionId": "sess-multi",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "id-1", "name": "Read",
						"input": map[string]any{"path": "/a"}},
					{"type": "tool_use", "id": "id-2", "name": "Bash",
						"input": map[string]any{"command": "ls"}},
				},
			},
		},
		{
			"uuid": "e2", "type": "user",
			"timestamp": "2026-04-10T10:00:05.000Z",
			"sessionId": "sess-multi",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "id-1", "content": "content of a"},
					{"type": "tool_result", "tool_use_id": "id-2", "content": "file.go"},
				},
			},
		},
	}

	writeFixtureJSONL(t, projDir, "sess-multi.jsonl", events)
	a := New(WithHomeDir(home))

	ch, err := a.StreamTurns("sess-multi")
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

	calls := turns[0].ToolCalls
	if len(calls) != 2 {
		t.Fatalf("ToolCalls len = %d, want 2", len(calls))
	}
	if calls[0].Output != "content of a" {
		t.Errorf("calls[0].Output = %q, want %q", calls[0].Output, "content of a")
	}
	if calls[1].Output != "file.go" {
		t.Errorf("calls[1].Output = %q, want %q", calls[1].Output, "file.go")
	}
}

func TestStreamTurns_ConsecutiveAssistantToolUse(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Real pattern: two separate assistant turns each with one tool_use,
	// followed by two user turns each with the matching tool_result.
	events := []map[string]any{
		{
			"uuid": "e1", "type": "assistant",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd": "/foo/bar", "sessionId": "sess-consec",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "id-a", "name": "Bash",
						"input": map[string]any{"command": "tlc task show 62"}},
				},
			},
		},
		{
			"uuid": "e2", "type": "assistant",
			"timestamp": "2026-04-10T10:00:01.000Z",
			"sessionId": "sess-consec",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "id-b", "name": "Bash",
						"input": map[string]any{"command": "xray map"}},
				},
			},
		},
		{
			"uuid": "e3", "type": "user",
			"timestamp": "2026-04-10T10:00:05.000Z",
			"sessionId": "sess-consec",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "id-a",
						"content": "task output"},
				},
			},
		},
		{
			"uuid": "e4", "type": "user",
			"timestamp": "2026-04-10T10:00:06.000Z",
			"sessionId": "sess-consec",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "id-b",
						"content": "xray output"},
				},
			},
		},
		{
			"uuid": "e5", "type": "user",
			"timestamp": "2026-04-10T10:01:00.000Z",
			"sessionId": "sess-consec",
			"message": map[string]any{
				"role":    "user",
				"content": "looks good",
			},
		},
	}

	writeFixtureJSONL(t, projDir, "sess-consec.jsonl", events)
	a := New(WithHomeDir(home))

	ch, err := a.StreamTurns("sess-consec")
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}
	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}

	// 2 assistant + 2 tool_result user + 1 plain user = 5
	if len(turns) != 5 {
		t.Fatalf("got %d turns, want 5", len(turns))
	}

	// Order: assistant(resolved) → user(tool_result) → assistant(resolved) → user(tool_result) → user.
	if len(turns[0].ToolCalls) == 0 {
		t.Fatal("turns[0] has no ToolCalls")
	}
	if turns[0].ToolCalls[0].Output != "task output" {
		t.Errorf("turns[0].ToolCalls[0].Output = %q, want %q",
			turns[0].ToolCalls[0].Output, "task output")
	}
	if len(turns[2].ToolCalls) == 0 {
		t.Fatal("turns[2] has no ToolCalls")
	}
	if turns[2].ToolCalls[0].Output != "xray output" {
		t.Errorf("turns[2].ToolCalls[0].Output = %q, want %q",
			turns[2].ToolCalls[0].Output, "xray output")
	}
}

func TestResumeCmd(t *testing.T) {
	a := New()
	got := a.ResumeCmd("abc-123")
	want := []string{"claude", "--resume", "abc-123"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ResumeCmd()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStreamTurns_SidechainResolved(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	const taskID = "toolu_xyz"
	events := []map[string]any{
		// main user turn
		{
			"uuid": "m1", "type": "user",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd":       "/foo/bar", "sessionId": "sess-sc",
			"message": map[string]any{
				"role":    "user",
				"content": "dispatch a sub-agent",
			},
		},
		// main assistant w/ Task tool_use
		{
			"uuid": "m2", "type": "assistant",
			"timestamp": "2026-04-10T10:00:01.000Z",
			"sessionId": "sess-sc",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Dispatching..."},
					{"type": "tool_use", "id": taskID, "name": "Task",
						"input": map[string]any{"prompt": "do work"}},
				},
			},
		},
		// 3 sidechain turns (parentToolUseID matches)
		{
			"uuid": "sc1", "type": "user",
			"timestamp":       "2026-04-10T10:00:02.000Z",
			"sessionId":       "sess-sc",
			"isSidechain":     true,
			"parentToolUseID": taskID,
			"message": map[string]any{
				"role": "user", "content": "sub task prompt",
			},
		},
		{
			"uuid": "sc2", "type": "assistant",
			"timestamp":       "2026-04-10T10:00:03.000Z",
			"sessionId":       "sess-sc",
			"isSidechain":     true,
			"parentToolUseID": taskID,
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "sub working"}},
			},
		},
		{
			"uuid": "sc3", "type": "assistant",
			"timestamp":       "2026-04-10T10:00:04.000Z",
			"sessionId":       "sess-sc",
			"isSidechain":     true,
			"parentToolUseID": taskID,
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "sub done"}},
			},
		},
		// main user resume
		{
			"uuid": "m3", "type": "user",
			"timestamp": "2026-04-10T10:00:05.000Z",
			"sessionId": "sess-sc",
			"message": map[string]any{
				"role":    "user",
				"content": "what did it find?",
			},
		},
		// main assistant final
		{
			"uuid": "m4", "type": "assistant",
			"timestamp": "2026-04-10T10:00:06.000Z",
			"sessionId": "sess-sc",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "summary"}},
			},
		},
	}

	writeFixtureJSONL(t, projDir, "sess-sc.jsonl", events)
	a := New(WithHomeDir(home))

	ch, err := a.StreamTurns("sess-sc")
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}
	var turns []session.Turn
	for tn := range ch {
		turns = append(turns, tn)
	}

	// Order: main-user, main-assistant(Task), sc1, sc2, sc3,
	// main-user-resume, main-assistant-final = 7.
	if len(turns) != 7 {
		t.Fatalf("got %d turns, want 7\nturns: %+v", len(turns), turns)
	}

	want := []struct {
		role            session.Role
		subtype         string
		contentContains string
	}{
		{session.RoleUser, "", "dispatch"},
		{session.RoleAssistant, "", "Dispatching"},
		{session.RoleUser, "sidechain", "sub task prompt"},
		{session.RoleAssistant, "sidechain", "sub working"},
		{session.RoleAssistant, "sidechain", "sub done"},
		{session.RoleUser, "", "what did it find"},
		{session.RoleAssistant, "", "summary"},
	}
	for i, w := range want {
		got := turns[i]
		if got.Role != w.role {
			t.Errorf("turn[%d].Role = %q, want %q", i, got.Role, w.role)
		}
		if got.Subtype != w.subtype {
			t.Errorf("turn[%d].Subtype = %q, want %q", i, got.Subtype, w.subtype)
		}
		if w.contentContains != "" && !contains(got.Content, w.contentContains) {
			t.Errorf("turn[%d].Content %q lacks %q", i, got.Content, w.contentContains)
		}
	}

	// Sidechain turns carry parent_tool_use_id.
	for _, i := range []int{2, 3, 4} {
		got, _ := turns[i].Metadata[sidechainMetaKey].(string)
		if got != taskID {
			t.Errorf("turn[%d].Metadata[%q] = %q, want %q",
				i, sidechainMetaKey, got, taskID)
		}
	}

	// TurnCount counts every type=user|assistant event once. Fixture
	// has 7 such events; sidechain entries must not double-count.
	s, err := a.GetSession("sess-sc")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.TurnCount != 7 {
		t.Errorf("TurnCount = %d, want 7", s.TurnCount)
	}
}

func TestStreamTurns_SidechainOrphanPreserved(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	events := []map[string]any{
		{
			"uuid": "u1", "type": "user",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd":       "/foo/bar", "sessionId": "sess-orphan",
			"message": map[string]any{"role": "user", "content": "hi"},
		},
		// orphan sidechain — parent tool id never appears as a Task call
		{
			"uuid": "sc1", "type": "assistant",
			"timestamp":       "2026-04-10T10:00:01.000Z",
			"sessionId":       "sess-orphan",
			"isSidechain":     true,
			"parentToolUseID": "toolu_orphan",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "orphan body"}},
			},
		},
		{
			"uuid": "u2", "type": "user",
			"timestamp": "2026-04-10T10:00:02.000Z",
			"sessionId": "sess-orphan",
			"message":   map[string]any{"role": "user", "content": "bye"},
		},
	}

	writeFixtureJSONL(t, projDir, "sess-orphan.jsonl", events)
	a := New(WithHomeDir(home))

	ch, err := a.StreamTurns("sess-orphan")
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}
	var turns []session.Turn
	for tn := range ch {
		turns = append(turns, tn)
	}

	if len(turns) != 3 {
		t.Fatalf("got %d turns, want 3", len(turns))
	}
	if turns[1].Subtype != "sidechain" {
		t.Errorf("turn[1].Subtype = %q, want sidechain", turns[1].Subtype)
	}
	if got, _ := turns[1].Metadata[sidechainMetaKey].(string); got != "toolu_orphan" {
		t.Errorf("orphan parent_tool_use_id = %q, want toolu_orphan", got)
	}
}

// usageEvents builds a JSONL fixture exercising message.usage on
// assistant turns. Two assistant events to verify accumulation.
func usageEvents(model string) []map[string]any {
	return []map[string]any{
		{
			"uuid": "u1", "type": "user",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd":       "/foo/bar",
			"sessionId": "sess-usage", "version": "2.1.112",
			"message": map[string]any{
				"role": "user", "content": "hi",
			},
		},
		{
			"uuid": "a1", "type": "assistant",
			"timestamp": "2026-04-10T10:00:05.000Z",
			"sessionId": "sess-usage", "version": "2.1.112",
			"message": map[string]any{
				"role": "assistant", "model": model,
				"content": []map[string]any{
					{"type": "text", "text": "first"},
				},
				"usage": map[string]any{
					"input_tokens":                10,
					"output_tokens":               40,
					"cache_read_input_tokens":     100,
					"cache_creation_input_tokens": 200,
				},
			},
		},
		{
			"uuid": "a2", "type": "assistant",
			"timestamp": "2026-04-10T10:00:10.000Z",
			"sessionId": "sess-usage", "version": "2.1.112",
			"message": map[string]any{
				"role": "assistant", "model": model,
				"content": []map[string]any{
					{"type": "text", "text": "second"},
				},
				"usage": map[string]any{
					"input_tokens":                5,
					"output_tokens":               20,
					"cache_read_input_tokens":     50,
					"cache_creation_input_tokens": 0,
				},
			},
		},
	}
}

func TestParseSession_UsageTelemetry(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureJSONL(t, projDir, "sess-usage.jsonl",
		usageEvents("claude-opus-4-7"))

	a := New(WithHomeDir(home))
	s, err := a.GetSession("sess-usage")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if got := s.Metadata["usage.tokens.input"]; got != 15 {
		t.Errorf("usage.tokens.input = %v, want 15", got)
	}
	if got := s.Metadata["usage.tokens.output"]; got != 60 {
		t.Errorf("usage.tokens.output = %v, want 60", got)
	}
	if got := s.Metadata["usage.tokens.cache_read"]; got != 150 {
		t.Errorf("usage.tokens.cache_read = %v, want 150", got)
	}
	if got := s.Metadata["usage.tokens.cache_write"]; got != 200 {
		t.Errorf("usage.tokens.cache_write = %v, want 200", got)
	}
	if got := s.Metadata["assistant.model"]; got != "claude-opus-4-7" {
		t.Errorf("assistant.model = %v, want claude-opus-4-7", got)
	}
	if got := s.Metadata["cli_version"]; got != "2.1.112" {
		t.Errorf("cli_version = %v, want 2.1.112", got)
	}
	cost, ok := s.Metadata["usage.cost_usd"].(float64)
	if !ok {
		t.Fatalf("usage.cost_usd missing or wrong type: %T",
			s.Metadata["usage.cost_usd"])
	}
	// in=15*15/M out=60*75/M creads=150*15*0.1/M cwrites=200*15*1.25/M
	wantCost := (15.0*15 + 60.0*75 + 150.0*15*0.1 + 200.0*15*1.25) /
		1_000_000
	if abs(cost-wantCost) > 1e-9 {
		t.Errorf("usage.cost_usd = %v, want %v", cost, wantCost)
	}
	if _, ok := s.Metadata["performance.duration_ms"].(int64); !ok {
		t.Errorf("performance.duration_ms missing or wrong type: %T",
			s.Metadata["performance.duration_ms"])
	}
}

func TestParseSession_UsageTelemetry_UnknownModel(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureJSONL(t, projDir, "sess-unknown.jsonl",
		usageEvents("claude-mystery-9-9"))

	a := New(WithHomeDir(home))
	s, err := a.GetSession("sess-unknown")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if _, ok := s.Metadata["usage.cost_usd"]; ok {
		t.Errorf("usage.cost_usd should be absent for unknown model, got %v",
			s.Metadata["usage.cost_usd"])
	}
	if got := s.Metadata["assistant.model"]; got != "claude-mystery-9-9" {
		t.Errorf("assistant.model = %v, want claude-mystery-9-9", got)
	}
}

func TestParseSession_NoUsageBlockDoesNotPanic(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal session: user + assistant turn, no usage block.
	events := []map[string]any{
		{
			"uuid": "u1", "type": "user",
			"timestamp": "2026-04-10T10:00:00.000Z",
			"cwd":       "/foo/bar", "sessionId": "sess-nousage",
			"message": map[string]any{
				"role": "user", "content": "hi",
			},
		},
		{
			"uuid": "a1", "type": "assistant",
			"timestamp": "2026-04-10T10:00:05.000Z",
			"sessionId": "sess-nousage",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "ok"},
				},
			},
		},
	}
	writeFixtureJSONL(t, projDir, "sess-nousage.jsonl", events)

	a := New(WithHomeDir(home))
	s, err := a.GetSession("sess-nousage")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	for _, k := range []string{
		"usage.tokens.input", "usage.tokens.output",
		"usage.tokens.cache_read", "usage.tokens.cache_write",
		"usage.cost_usd", "assistant.model",
	} {
		if _, ok := s.Metadata[k]; ok {
			t.Errorf("metadata[%q] = %v, want absent", k, s.Metadata[k])
		}
	}
}

func TestStreamTurns_PerTurnOutputTokens(t *testing.T) {
	home := t.TempDir()
	projDir := filepath.Join(home, ".claude", "projects", "-foo-bar")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureJSONL(t, projDir, "sess-usage.jsonl",
		usageEvents("claude-opus-4-7"))

	a := New(WithHomeDir(home))
	ch, err := a.StreamTurns("sess-usage")
	if err != nil {
		t.Fatalf("StreamTurns: %v", err)
	}
	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}
	// 1 user + 2 assistant.
	if len(turns) != 3 {
		t.Fatalf("got %d turns, want 3", len(turns))
	}
	if got := turns[1].Metadata["usage.tokens.output"]; got != 40 {
		t.Errorf("turns[1] output tokens = %v, want 40", got)
	}
	if got := turns[2].Metadata["usage.tokens.output"]; got != 20 {
		t.Errorf("turns[2] output tokens = %v, want 20", got)
	}
	if turns[0].Metadata != nil {
		t.Errorf("user turn metadata = %v, want nil", turns[0].Metadata)
	}
}

func TestCostUSD_KnownModelMatchesFormula(t *testing.T) {
	cost, ok := costUSD("claude-sonnet-4-6", 1_000_000, 1_000_000, 0, 0)
	if !ok {
		t.Fatal("expected ok=true for sonnet-4-6")
	}
	if cost != 18.0 {
		t.Errorf("cost = %v, want 18.0 (3 input + 15 output per 1M)",
			cost)
	}
}

func TestCostUSD_UnknownModelReturnsFalse(t *testing.T) {
	if _, ok := costUSD("claude-mystery", 100, 100, 0, 0); ok {
		t.Error("expected ok=false for unknown model")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// contains is a small substring helper to avoid importing strings into
// every test for assertions; mirrors strings.Contains.
func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

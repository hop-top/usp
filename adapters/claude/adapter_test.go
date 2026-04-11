package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// Compile-time interface satisfaction check.
var _ session.SessionAdapter = (*Adapter)(nil)

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
	if s.ID != "sess-abc" {
		t.Errorf("ID = %q, want %q", s.ID, "sess-abc")
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
	if s.ID != "sess-abc" {
		t.Errorf("ID = %q, want %q", s.ID, "sess-abc")
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

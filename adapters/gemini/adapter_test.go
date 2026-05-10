package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

// writeProjectsJSON creates a projects.json fixture in tmpHome/.gemini/.
func writeProjectsJSON(t *testing.T, tmpHome string, projects map[string]string) {
	t.Helper()
	root := filepath.Join(tmpHome, ".gemini")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(projectsJSON{Projects: projects})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkHistoryDir creates ~/.gemini/tmp/<alias>/chats with a .project_root marker.
func mkHistoryDir(t *testing.T, tmpHome, alias, cwd string) {
	t.Helper()
	dir := filepath.Join(tmpHome, ".gemini", "tmp", alias)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, ".project_root"), []byte(cwd), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "chats"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeChatJSONL(t *testing.T, tmpHome, alias, sessionID string, lines ...any) {
	t.Helper()
	chatDir := filepath.Join(tmpHome, ".gemini", "tmp", alias, "chats")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	all := []any{chatHeader{
		SessionID:   sessionID,
		ProjectHash: "hash",
		StartTime:   "2026-05-09T12:00:00Z",
		LastUpdated: "2026-05-09T12:00:00Z",
		Kind:        "main",
	}}
	all = append(all, lines...)
	if err := writeJSONLines(
		filepath.Join(chatDir, "session-2026-05-09T12-00-"+sessionID[:8]+".jsonl"),
		all,
	); err != nil {
		t.Fatal(err)
	}
}

func TestInterfaceSatisfaction(t *testing.T) {
	var _ session.SessionAdapter = (*Adapter)(nil)
}

func TestCLI(t *testing.T) {
	a := &Adapter{}
	if got := a.CLI(); got != uxp.CLIGemini {
		t.Fatalf("CLI() = %q, want %q", got, uxp.CLIGemini)
	}
}

func TestProjectKey_FromMap(t *testing.T) {
	tmp := t.TempDir()
	cwd := "/Users/me/projects/myapp"
	writeProjectsJSON(t, tmp, map[string]string{
		cwd: "myapp",
	})

	a := &Adapter{HomeDir: tmp}
	got := a.ProjectKey(cwd)
	if got != "myapp" {
		t.Fatalf("ProjectKey() = %q, want %q", got, "myapp")
	}
}

func TestProjectKey_Fallback(t *testing.T) {
	tmp := t.TempDir()
	// No projects.json — should fall back to basename.
	a := &Adapter{HomeDir: tmp}
	got := a.ProjectKey("/some/path/coolproject")
	if got != "coolproject" {
		t.Fatalf("ProjectKey() = %q, want %q", got, "coolproject")
	}
}

func TestProjectKey_CollisionAlias(t *testing.T) {
	tmp := t.TempDir()
	writeProjectsJSON(t, tmp, map[string]string{
		"/a/hops/main": "main",
		"/b/hops/main": "main-1",
		"/c/hops/main": "main-2",
	})

	a := &Adapter{HomeDir: tmp}
	if got := a.ProjectKey("/b/hops/main"); got != "main-1" {
		t.Fatalf("ProjectKey() = %q, want %q", got, "main-1")
	}
}

func TestListSessions_EmptyHistoryDir(t *testing.T) {
	tmp := t.TempDir()
	cwd := "/Users/me/projects/myapp"
	writeProjectsJSON(t, tmp, map[string]string{cwd: "myapp"})
	mkHistoryDir(t, tmp, "myapp", cwd)

	a := &Adapter{HomeDir: tmp}
	sessions, err := a.ListSessions(cwd)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("ListSessions() returned %d sessions, want 0 (only .project_root present)",
			len(sessions))
	}
}

func TestListSessions_NonExistentDir(t *testing.T) {
	tmp := t.TempDir()
	writeProjectsJSON(t, tmp, map[string]string{"/x": "x"})

	a := &Adapter{HomeDir: tmp}
	sessions, err := a.ListSessions("/x")
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if sessions != nil {
		t.Fatalf("ListSessions() = %v, want nil for missing dir", sessions)
	}
}

func TestListSessions_WithChatFiles(t *testing.T) {
	tmp := t.TempDir()
	cwd := "/Users/me/projects/myapp"
	writeProjectsJSON(t, tmp, map[string]string{cwd: "myapp"})
	mkHistoryDir(t, tmp, "myapp", cwd)

	writeChatJSONL(t, tmp, "myapp", "11111111-1111-4111-8111-111111111111")
	writeChatJSONL(t, tmp, "myapp", "22222222-2222-4222-8222-222222222222")

	a := &Adapter{HomeDir: tmp}
	sessions, err := a.ListSessions(cwd)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("ListSessions() returned %d sessions, want 2", len(sessions))
	}

	ids := map[string]bool{}
	for _, s := range sessions {
		ids[s.NativeID] = true
		if s.CLI != uxp.CLIGemini {
			t.Errorf("session CLI = %q, want %q", s.CLI, uxp.CLIGemini)
		}
		if s.ProjectCwd != cwd {
			t.Errorf("session ProjectCwd = %q, want %q", s.ProjectCwd, cwd)
		}
	}
	if !ids["11111111-1111-4111-8111-111111111111"] ||
		!ids["22222222-2222-4222-8222-222222222222"] {
		t.Errorf("unexpected native session IDs: %v", ids)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	tmp := t.TempDir()
	writeProjectsJSON(t, tmp, map[string]string{"/x": "x"})
	mkHistoryDir(t, tmp, "x", "/x")

	a := &Adapter{HomeDir: tmp}
	_, err := a.GetSession("nonexistent")
	if err == nil {
		t.Fatal("GetSession() expected error for missing transcript")
	}
}

func TestGetSession_Found(t *testing.T) {
	tmp := t.TempDir()
	cwd := "/Users/me/proj"
	writeProjectsJSON(t, tmp, map[string]string{cwd: "proj"})
	mkHistoryDir(t, tmp, "proj", cwd)
	writeChatJSONL(t, tmp, "proj", "33333333-3333-4333-8333-333333333333")

	a := &Adapter{HomeDir: tmp}
	s, err := a.GetSession("33333333-3333-4333-8333-333333333333")
	if err != nil {
		t.Fatalf("GetSession() error: %v", err)
	}
	if s.NativeID != "33333333-3333-4333-8333-333333333333" {
		t.Errorf("NativeID = %q", s.NativeID)
	}
	if s.ProjectCwd != cwd {
		t.Errorf("ProjectCwd = %q, want %q", s.ProjectCwd, cwd)
	}
}

func TestStreamTurns_NotFound(t *testing.T) {
	tmp := t.TempDir()
	writeProjectsJSON(t, tmp, map[string]string{"/x": "x"})

	a := &Adapter{HomeDir: tmp}
	_, err := a.StreamTurns("missing")
	if err == nil {
		t.Fatal("StreamTurns() expected error for missing transcript")
	}
}

func TestStreamTurns_ParsesHistory(t *testing.T) {
	tmp := t.TempDir()
	cwd := "/Users/me/proj"
	writeProjectsJSON(t, tmp, map[string]string{cwd: "proj"})
	mkHistoryDir(t, tmp, "proj", cwd)

	writeChatJSONL(t, tmp, "proj", "44444444-4444-4444-8444-444444444444",
		map[string]any{"type": "user", "timestamp": "2026-05-09T12:00:01Z",
			"content": []chatContentPart{{Text: "hello"}}},
		map[string]any{"type": "gemini", "timestamp": "2026-05-09T12:00:02Z",
			"content": "hi there"},
		map[string]any{"type": "user", "timestamp": "2026-05-09T12:00:03Z",
			"content": []chatContentPart{{Text: "thanks"}}},
	)

	a := &Adapter{HomeDir: tmp}
	ch, err := a.StreamTurns("44444444-4444-4444-8444-444444444444")
	if err != nil {
		t.Fatalf("StreamTurns() error: %v", err)
	}

	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}
	if len(turns) != 3 {
		t.Fatalf("got %d turns, want 3", len(turns))
	}
	if turns[0].Role != session.RoleUser || turns[0].Content != "hello" {
		t.Errorf("turn[0] = %+v", turns[0])
	}
	if turns[1].Role != session.RoleAssistant || turns[1].Content != "hi there" {
		t.Errorf("turn[1] = %+v, want assistant/hi there", turns[1])
	}
}

func TestCapabilities(t *testing.T) {
	a := &Adapter{}
	caps := a.Capabilities()

	// Native
	if !caps.Supports("per-project-sessions") {
		t.Error("expected native support for per-project-sessions")
	}
	if !caps.Supports("mcp-servers") {
		t.Error("expected native support for mcp-servers")
	}

	// Workaround
	if !caps.Supports("transcript-read") {
		t.Error("expected workaround support for transcript-read")
	}

	// Missing
	if caps.Supports("memory-subsystem") {
		t.Error("expected missing for memory-subsystem")
	}
	if caps.Supports("append-stream") {
		t.Error("expected missing for append-stream")
	}

	// Coverage count: 10 native + 4 workaround + 6 missing = 20
	cov := caps.Coverage()
	var native, workaround, missing int
	for _, s := range cov {
		switch s {
		case uxp.Native:
			native++
		case uxp.Workaround:
			workaround++
		case uxp.Missing:
			missing++
		}
	}
	if native != 10 {
		t.Errorf("native count = %d, want 10", native)
	}
	if workaround != 4 {
		t.Errorf("workaround count = %d, want 4", workaround)
	}
	if missing != 6 {
		t.Errorf("missing count = %d, want 6", missing)
	}
}

func TestResumeAdapterInterface(t *testing.T) {
	var _ session.ResumeAdapter = (*Adapter)(nil)
}

func TestGetSession_AssistantModelFromTranscript(t *testing.T) {
	tmp := t.TempDir()
	cwd := "/Users/me/proj"
	writeProjectsJSON(t, tmp, map[string]string{cwd: "proj"})
	mkHistoryDir(t, tmp, "proj", cwd)

	writeChatJSONL(t, tmp, "proj", "55555555-5555-4555-8555-555555555555",
		map[string]any{"type": "user", "content": []chatContentPart{{Text: "hi"}}},
		map[string]any{"type": "gemini", "content": "hello", "model": "gemini-3-pro"},
	)

	a := &Adapter{HomeDir: tmp}
	s, err := a.GetSession("55555555-5555-4555-8555-555555555555")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got := s.Metadata["assistant.model"]; got != "gemini-3-pro" {
		t.Errorf("assistant.model = %v, want gemini-3-pro", got)
	}
}

func TestGetSession_AssistantModelAbsent(t *testing.T) {
	tmp := t.TempDir()
	cwd := "/Users/me/proj"
	writeProjectsJSON(t, tmp, map[string]string{cwd: "proj"})
	mkHistoryDir(t, tmp, "proj", cwd)

	writeChatJSONL(t, tmp, "proj", "66666666-6666-4666-8666-666666666666")

	a := &Adapter{HomeDir: tmp}
	s, err := a.GetSession("66666666-6666-4666-8666-666666666666")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if _, ok := s.Metadata["assistant.model"]; ok {
		t.Errorf("assistant.model should be absent, got %v",
			s.Metadata["assistant.model"])
	}
}

func TestResumeCmd(t *testing.T) {
	a := &Adapter{}
	got := a.ResumeCmd("my-tag")
	want := []string{"gemini", "--resume", "my-tag"}
	if len(got) != len(want) {
		t.Fatalf("ResumeCmd() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ResumeCmd()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestInjectSession(t *testing.T) {
	tmp := t.TempDir()
	cwd := "/Users/me/projects/myapp"
	writeProjectsJSON(t, tmp, map[string]string{cwd: "myapp"})

	a := &Adapter{HomeDir: tmp}
	turns := []session.Turn{
		{Role: session.RoleUser, Content: "hello", Timestamp: time.Now()},
		{Role: session.RoleAssistant, Content: "hi there", Timestamp: time.Now(),
			ToolCalls: []session.ToolCall{
				{Name: "Read", Output: "file contents"},
			}},
		{Role: session.RoleSystem, Content: "context info", Timestamp: time.Now()},
	}

	sessionID, err := a.InjectSession(cwd, turns)
	if err != nil {
		t.Fatalf("InjectSession() error: %v", err)
	}
	if len(sessionID) != len("00000000-0000-0000-0000-000000000000") {
		t.Fatalf("sessionID %q unexpected length", sessionID)
	}

	// Verify file was created in the right place.
	chatDir := filepath.Join(tmp, ".gemini", "tmp", "myapp", "chats")
	entries, err := os.ReadDir(chatDir)
	if err != nil {
		t.Fatalf("read injected chat dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("chat files = %d, want 1", len(entries))
	}
	lines, err := readJSONLines(filepath.Join(chatDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read injected file: %v", err)
	}
	header, err := readChatHeader(filepath.Join(chatDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	if header.SessionID != sessionID {
		t.Fatalf("header sessionID = %q, want %q", header.SessionID, sessionID)
	}
	if len(lines) != 7 {
		t.Fatalf("jsonl lines = %d, want 7", len(lines))
	}
	turn0, ok := chatLineToTurn(lines[1])
	if !ok || turn0.Role != session.RoleUser || turn0.Content != "hello" {
		t.Errorf("line[1] turn = %+v ok=%v", turn0, ok)
	}
	turn1, ok := chatLineToTurn(lines[3])
	if !ok || turn1.Role != session.RoleAssistant ||
		!strings.Contains(turn1.Content, "[Tool: Read") {
		t.Errorf("line[3] turn = %+v ok=%v", turn1, ok)
	}
	turn2, ok := chatLineToTurn(lines[5])
	if !ok || turn2.Role != session.RoleUser ||
		!strings.HasPrefix(turn2.Content, "[System] ") {
		t.Errorf("line[5] turn = %+v ok=%v", turn2, ok)
	}

	// Verify ListSessions finds the injected session.
	sessions, err := a.ListSessions(cwd)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.NativeID == sessionID {
			found = true
		}
	}
	if !found {
		t.Errorf("ListSessions() did not return injected session %q", sessionID)
	}

	// Verify GetSession finds it.
	s, err := a.GetSession(sessionID)
	if err != nil {
		t.Fatalf("GetSession(%q) error: %v", sessionID, err)
	}
	if s.NativeID != sessionID || s.CLI != uxp.CLIGemini {
		t.Errorf("GetSession() = %+v", s)
	}

	// Verify StreamTurns reads back the parts-based format.
	ch, err := a.StreamTurns(sessionID)
	if err != nil {
		t.Fatalf("StreamTurns(%q) error: %v", sessionID, err)
	}
	var readBack []session.Turn
	for turn := range ch {
		readBack = append(readBack, turn)
	}
	if len(readBack) != 3 {
		t.Fatalf("StreamTurns() returned %d turns, want 3", len(readBack))
	}
	if readBack[0].Role != session.RoleUser || readBack[0].Content != "hello" {
		t.Errorf("turn[0] = %+v", readBack[0])
	}
	if readBack[1].Role != session.RoleAssistant {
		t.Errorf("turn[1].Role = %q, want assistant", readBack[1].Role)
	}
}

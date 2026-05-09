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

// mkHistoryDir creates ~/.gemini/history/<alias>/ with a .project_root marker.
func mkHistoryDir(t *testing.T, tmpHome, alias, cwd string) {
	t.Helper()
	dir := filepath.Join(tmpHome, ".gemini", "history", alias)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, ".project_root"), []byte(cwd), 0o644,
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
		"/a/hops/main":  "main",
		"/b/hops/main":  "main-1",
		"/c/hops/main":  "main-2",
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

	// Write mock chat files (for when Gemini starts writing them).
	histDir := filepath.Join(tmp, ".gemini", "history", "myapp")
	for _, tag := range []string{"refactor-db", "add-auth"} {
		data := []byte(`{"history":[]}`)
		if err := os.WriteFile(
			filepath.Join(histDir, tag+".json"), data, 0o644,
		); err != nil {
			t.Fatal(err)
		}
	}

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
	if !ids["refactor-db"] || !ids["add-auth"] {
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

	histDir := filepath.Join(tmp, ".gemini", "history", "proj")
	if err := os.WriteFile(
		filepath.Join(histDir, "my-chat.json"),
		[]byte(`{"history":[]}`), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{HomeDir: tmp}
	s, err := a.GetSession("my-chat")
	if err != nil {
		t.Fatalf("GetSession() error: %v", err)
	}
	if s.NativeID != "my-chat" {
		t.Errorf("NativeID = %q, want %q", s.NativeID, "my-chat")
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

	chat := map[string]any{
		"history": []map[string]string{
			{"role": "user", "content": "hello"},
			{"role": "model", "content": "hi there"},
			{"role": "user", "content": "thanks"},
		},
	}
	data, _ := json.Marshal(chat)
	histDir := filepath.Join(tmp, ".gemini", "history", "proj")
	if err := os.WriteFile(
		filepath.Join(histDir, "convo.json"), data, 0o644,
	); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{HomeDir: tmp}
	ch, err := a.StreamTurns("convo")
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

	// Coverage count: 9 native + 4 workaround + 7 missing = 20
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
	if native != 9 {
		t.Errorf("native count = %d, want 9", native)
	}
	if workaround != 4 {
		t.Errorf("workaround count = %d, want 4", workaround)
	}
	if missing != 7 {
		t.Errorf("missing count = %d, want 7", missing)
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

	histDir := filepath.Join(tmp, ".gemini", "history", "proj")
	chat := map[string]any{
		"model": "gemini-3-pro",
		"history": []map[string]any{
			{"role": "user", "content": "hi"},
			{"role": "model", "content": "hello"},
		},
	}
	data, _ := json.Marshal(chat)
	if err := os.WriteFile(
		filepath.Join(histDir, "with-model.json"), data, 0o644,
	); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{HomeDir: tmp}
	s, err := a.GetSession("with-model")
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

	histDir := filepath.Join(tmp, ".gemini", "history", "proj")
	if err := os.WriteFile(
		filepath.Join(histDir, "no-model.json"),
		[]byte(`{"history":[]}`), 0o644,
	); err != nil {
		t.Fatal(err)
	}

	a := &Adapter{HomeDir: tmp}
	s, err := a.GetSession("no-model")
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

	tag, err := a.InjectSession(cwd, turns)
	if err != nil {
		t.Fatalf("InjectSession() error: %v", err)
	}
	if !strings.HasPrefix(tag, "usp-resume-") {
		t.Fatalf("tag %q missing usp-resume- prefix", tag)
	}
	if len(tag) != len("usp-resume-")+8 {
		t.Fatalf("tag %q unexpected length", tag)
	}

	// Verify file was created in the right place.
	p := filepath.Join(tmp, ".gemini", "history", "myapp", tag+".json")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read injected file: %v", err)
	}

	var chat geminiChat
	if err := json.Unmarshal(data, &chat); err != nil {
		t.Fatalf("parse injected file: %v", err)
	}
	if len(chat.History) != 3 {
		t.Fatalf("history len = %d, want 3", len(chat.History))
	}
	if chat.History[0].Role != "user" || chat.History[0].Parts[0].Text != "hello" {
		t.Errorf("msg[0] = %+v", chat.History[0])
	}
	if chat.History[1].Role != "model" {
		t.Errorf("msg[1].role = %q, want model", chat.History[1].Role)
	}
	if !strings.Contains(chat.History[1].Parts[0].Text, "[Tool: Read") {
		t.Errorf("msg[1] missing tool summary: %s", chat.History[1].Parts[0].Text)
	}
	if chat.History[2].Role != "user" {
		t.Errorf("msg[2].role = %q, want user (system mapped)", chat.History[2].Role)
	}
	if !strings.HasPrefix(chat.History[2].Parts[0].Text, "[System] ") {
		t.Errorf("msg[2] missing [System] prefix: %s", chat.History[2].Parts[0].Text)
	}

	// Verify ListSessions finds the injected session.
	sessions, err := a.ListSessions(cwd)
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.NativeID == tag {
			found = true
		}
	}
	if !found {
		t.Errorf("ListSessions() did not return injected session %q", tag)
	}

	// Verify GetSession finds it.
	s, err := a.GetSession(tag)
	if err != nil {
		t.Fatalf("GetSession(%q) error: %v", tag, err)
	}
	if s.NativeID != tag || s.CLI != uxp.CLIGemini {
		t.Errorf("GetSession() = %+v", s)
	}

	// Verify StreamTurns reads back the parts-based format.
	ch, err := a.StreamTurns(tag)
	if err != nil {
		t.Fatalf("StreamTurns(%q) error: %v", tag, err)
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

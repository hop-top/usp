package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hop.top/usp/session"
)

func writeJSONLForTest(t *testing.T, path string, events []map[string]any) {
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

func TestExtractSkillsSlashCommand(t *testing.T) {
	home := t.TempDir()
	a := New(WithHomeDir(home))
	key := a.ProjectKey("/tmp/skills-test")
	dir := filepath.Join(home, ".claude", "projects", key)

	writeJSONLForTest(t, filepath.Join(dir, "sk-01.jsonl"), []map[string]any{
		{
			"uuid": "u1", "type": "user",
			"timestamp": "2026-04-12T09:00:00Z",
			"cwd":       "/tmp/skills-test", "sessionId": "sk-01",
			"message": map[string]any{
				"role": "user",
				"content": "<command-message>retro</command-message>\n" +
					"<command-name>/retro</command-name>\n" +
					"<command-args>review last week</command-args>",
			},
		},
		{
			"uuid": "a1", "type": "assistant",
			"timestamp": "2026-04-12T09:00:05Z",
			"sessionId": "sk-01",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "running retro"},
				},
			},
		},
	})

	got, err := a.ExtractSkills("sk-01")
	if err != nil {
		t.Fatalf("ExtractSkills: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	e := got[0]
	if e.Name != "retro" {
		t.Errorf("Name = %q, want %q", e.Name, "retro")
	}
	if e.TriggerTurnID != "u1" {
		t.Errorf("TriggerTurnID = %q, want u1", e.TriggerTurnID)
	}
	if e.TriggerQuery != "review last week" {
		t.Errorf("TriggerQuery = %q", e.TriggerQuery)
	}
	if e.Outcome != session.SkillInvoked {
		t.Errorf("Outcome = %q, want invoked", e.Outcome)
	}
}

func TestExtractSkillsToolCall(t *testing.T) {
	home := t.TempDir()
	a := New(WithHomeDir(home))
	key := a.ProjectKey("/tmp/skills-test")
	dir := filepath.Join(home, ".claude", "projects", key)

	writeJSONLForTest(t, filepath.Join(dir, "sk-02.jsonl"), []map[string]any{
		{
			"uuid": "u1", "type": "user",
			"timestamp": "2026-04-12T10:00:00Z",
			"cwd":       "/tmp/skills-test", "sessionId": "sk-02",
			"message": map[string]any{
				"role":    "user",
				"content": "review the recent changes",
			},
		},
		{
			"uuid": "a1", "type": "assistant",
			"timestamp": "2026-04-12T10:00:01Z",
			"sessionId": "sk-02",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "toolu_1",
						"name":  "Skill",
						"input": map[string]any{"skill": "review"},
					},
				},
			},
		},
		{
			"uuid": "u2", "type": "user",
			"timestamp": "2026-04-12T10:00:02Z",
			"sessionId": "sk-02",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "toolu_1",
						"content":     "ok",
					},
				},
			},
		},
	})

	got, err := a.ExtractSkills("sk-02")
	if err != nil {
		t.Fatalf("ExtractSkills: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	e := got[0]
	if e.Name != "review" {
		t.Errorf("Name = %q, want review", e.Name)
	}
	if e.TriggerTurnID != "u1" {
		t.Errorf("TriggerTurnID = %q, want u1", e.TriggerTurnID)
	}
	if e.TriggerQuery != "review the recent changes" {
		t.Errorf("TriggerQuery = %q", e.TriggerQuery)
	}
	if e.Outcome != session.SkillInvoked {
		t.Errorf("Outcome = %q, want invoked", e.Outcome)
	}
}

func TestExtractSkillsErroredOutcome(t *testing.T) {
	home := t.TempDir()
	a := New(WithHomeDir(home))
	key := a.ProjectKey("/tmp/skills-test")
	dir := filepath.Join(home, ".claude", "projects", key)

	writeJSONLForTest(t, filepath.Join(dir, "sk-03.jsonl"), []map[string]any{
		{
			"uuid": "u1", "type": "user",
			"timestamp": "2026-04-12T11:00:00Z",
			"cwd":       "/tmp/skills-test", "sessionId": "sk-03",
			"message": map[string]any{"role": "user", "content": "broken"},
		},
		{
			"uuid": "a1", "type": "assistant",
			"timestamp": "2026-04-12T11:00:01Z",
			"sessionId": "sk-03",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "toolu_err",
						"name":  "Skill",
						"input": map[string]any{"skill": "ship"},
					},
				},
			},
		},
		{
			"uuid": "u2", "type": "user",
			"timestamp": "2026-04-12T11:00:02Z",
			"sessionId": "sk-03",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "toolu_err",
						"is_error":    true,
						"content":     "skill not found",
					},
				},
			},
		},
	})

	got, err := a.ExtractSkills("sk-03")
	if err != nil {
		t.Fatalf("ExtractSkills: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].Outcome != session.SkillErrored {
		t.Errorf("Outcome = %q, want errored", got[0].Outcome)
	}
}

func TestExtractSkillsNoSkills(t *testing.T) {
	home := t.TempDir()
	a := New(WithHomeDir(home))
	key := a.ProjectKey("/tmp/skills-test")
	dir := filepath.Join(home, ".claude", "projects", key)

	writeJSONLForTest(t, filepath.Join(dir, "sk-04.jsonl"), []map[string]any{
		{
			"uuid": "u1", "type": "user",
			"timestamp": "2026-04-12T12:00:00Z",
			"cwd":       "/tmp/skills-test", "sessionId": "sk-04",
			"message": map[string]any{"role": "user", "content": "hello"},
		},
		{
			"uuid": "a1", "type": "assistant",
			"timestamp": "2026-04-12T12:00:01Z",
			"sessionId": "sk-04",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "hi back"},
				},
			},
		},
	})

	got, err := a.ExtractSkills("sk-04")
	if err != nil {
		t.Fatalf("ExtractSkills: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d events, want 0", len(got))
	}
}

func TestParseSlashCommandEdgeCases(t *testing.T) {
	tests := []struct {
		in       string
		wantName string
		wantArgs string
		wantOK   bool
	}{
		{
			in:       "<command-name>/foo</command-name><command-args>bar</command-args>",
			wantName: "foo", wantArgs: "bar", wantOK: true,
		},
		{
			in:       "<command-name>foo</command-name>", // missing leading slash
			wantName: "foo", wantArgs: "", wantOK: true,
		},
		{
			in:     "no command here",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		name, args, ok := parseSlashCommand(tt.in)
		if ok != tt.wantOK {
			t.Errorf("ok = %v for %q, want %v", ok, tt.in, tt.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if name != tt.wantName {
			t.Errorf("name = %q, want %q", name, tt.wantName)
		}
		if args != tt.wantArgs {
			t.Errorf("args = %q, want %q", args, tt.wantArgs)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("0123456789abc", 10); got != "0123456789…" {
		t.Errorf("truncate long = %q", got)
	}
}

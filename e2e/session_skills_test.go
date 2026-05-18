package e2e

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"hop.top/usp/adapters/claude"
	"hop.top/usp/session"
)

// setupClaudeWithSkills writes a Claude transcript that exercises
// every skill-extraction code path:
//   - slash command (/retro) with args
//   - Skill tool_use that succeeds
//   - Skill tool_use that errors
//   - a plain assistant turn (no skill activity) for negative coverage
func setupClaudeWithSkills(t *testing.T, home string) *claude.Adapter {
	t.Helper()
	a := claude.New(claude.WithHomeDir(home))
	key := a.ProjectKey(fixtureCwd)
	dir := filepath.Join(home, ".claude", "projects", key)

	writeJSONL(t, filepath.Join(dir, "skills-sess-01.jsonl"), []map[string]any{
		{
			"uuid": "u1", "type": "user",
			"timestamp": "2026-04-12T09:00:00Z",
			"cwd":       fixtureCwd, "sessionId": "skills-sess-01",
			"message": map[string]any{
				"role": "user",
				"content": "<command-message>retro</command-message>\n" +
					"<command-name>/retro</command-name>\n" +
					"<command-args>review last week</command-args>",
			},
		},
		{
			"uuid": "a1", "type": "assistant",
			"timestamp": "2026-04-12T09:00:01Z",
			"sessionId": "skills-sess-01",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "running retro"},
				},
			},
		},
		{
			"uuid": "u2", "type": "user",
			"timestamp": "2026-04-12T09:01:00Z",
			"sessionId": "skills-sess-01",
			"message": map[string]any{
				"role":    "user",
				"content": "review my code",
			},
		},
		{
			"uuid": "a2", "type": "assistant",
			"timestamp": "2026-04-12T09:01:01Z",
			"sessionId": "skills-sess-01",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "toolu_review",
						"name":  "Skill",
						"input": map[string]any{"skill": "review"},
					},
				},
			},
		},
		{
			"uuid": "u3", "type": "user",
			"timestamp": "2026-04-12T09:01:02Z",
			"sessionId": "skills-sess-01",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "toolu_review",
						"content":     "review complete",
					},
				},
			},
		},
		{
			"uuid": "u4", "type": "user",
			"timestamp": "2026-04-12T09:02:00Z",
			"sessionId": "skills-sess-01",
			"message": map[string]any{
				"role":    "user",
				"content": "ship it",
			},
		},
		{
			"uuid": "a3", "type": "assistant",
			"timestamp": "2026-04-12T09:02:01Z",
			"sessionId": "skills-sess-01",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "toolu_ship",
						"name":  "Skill",
						"input": map[string]any{"skill": "ship"},
					},
				},
			},
		},
		{
			"uuid": "u5", "type": "user",
			"timestamp": "2026-04-12T09:02:02Z",
			"sessionId": "skills-sess-01",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "toolu_ship",
						"is_error":    true,
						"content":     "ship failed",
					},
				},
			},
		},
	})
	return a
}

// TestSessionSkillsExtractAndFilter verifies the AC matrix for
// US-0005 / T-0070 against a single Claude transcript carrying
// the three skill-event shapes (slash command, tool call,
// tool call with errored result).
func TestSessionSkillsExtractAndFilter(t *testing.T) {
	home := t.TempDir()
	a := setupClaudeWithSkills(t, home)

	events, err := a.ExtractSkills("skills-sess-01")
	if err != nil {
		t.Fatalf("ExtractSkills: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	wantNames := []string{"retro", "review", "ship"}
	wantOutcome := []session.SkillOutcome{
		session.SkillInvoked, session.SkillInvoked, session.SkillErrored,
	}
	for i, e := range events {
		if e.Name != wantNames[i] {
			t.Errorf("events[%d].Name = %q, want %q", i, e.Name, wantNames[i])
		}
		if e.Outcome != wantOutcome[i] {
			t.Errorf("events[%d].Outcome = %q, want %q",
				i, e.Outcome, wantOutcome[i])
		}
		if e.SessionID != "skills-sess-01" {
			t.Errorf("events[%d].SessionID = %q", i, e.SessionID)
		}
		if e.TriggerTurnID == "" {
			t.Errorf("events[%d] missing TriggerTurnID", i)
		}
		if e.TriggerQuery == "" {
			t.Errorf("events[%d] missing TriggerQuery", i)
		}
	}

	// --name filter equivalent: substring on Name.
	var filtered []session.SkillEvent
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.Name), "rev") {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) != 1 || filtered[0].Name != "review" {
		t.Errorf("--name filter expected single 'review', got %v", filtered)
	}
}

// TestSessionSkillsJSONRoundTrip exercises the documented schema:
// every SkillEvent must serialize+deserialize without loss when
// callers consume `--format json`.
func TestSessionSkillsJSONRoundTrip(t *testing.T) {
	home := t.TempDir()
	a := setupClaudeWithSkills(t, home)
	events, err := a.ExtractSkills("skills-sess-01")
	if err != nil {
		t.Fatalf("ExtractSkills: %v", err)
	}

	data, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got []session.SkillEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != len(events) {
		t.Fatalf("round-trip length mismatch: got %d, want %d",
			len(got), len(events))
	}
	for i := range got {
		if got[i].Name != events[i].Name {
			t.Errorf("name[%d] mismatch", i)
		}
		if got[i].Outcome != events[i].Outcome {
			t.Errorf("outcome[%d] mismatch", i)
		}
		if !got[i].Timestamp.Equal(events[i].Timestamp) {
			t.Errorf("timestamp[%d] mismatch", i)
		}
	}
}

// TestSessionSkillsUnsupportedAdapter verifies the contract for
// adapters that lack skill primitives: callers receive an
// `unsupported: true` row rather than a hard error. We assert at
// the type level — codex/gemini/opencode adapters intentionally
// do not implement [session.SkillExtractor].
func TestSessionSkillsUnsupportedAdapter(t *testing.T) {
	home := t.TempDir()
	codexA := setupCodex(t, home)
	geminiA := setupGemini(t, home)
	openA := setupOpenCode(t, home)

	for _, a := range []session.SessionAdapter{codexA, geminiA, openA} {
		if _, ok := a.(session.SkillExtractor); ok {
			t.Errorf("%s adapter should NOT implement SkillExtractor "+
				"(skill primitives unsupported)", a.CLI())
		}
	}

	// Claude DOES implement it.
	claudeA := setupClaude(t, home)
	if _, ok := session.SessionAdapter(claudeA).(session.SkillExtractor); !ok {
		t.Error("claude adapter should implement SkillExtractor")
	}
}

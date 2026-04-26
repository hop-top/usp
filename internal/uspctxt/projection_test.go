package uspctxt

import (
	"strings"
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

func sampleSession() session.Session {
	end := time.Date(2026, 4, 25, 12, 30, 0, 0, time.UTC)
	return session.Session{
		ID:         "fe2eb947-ecab-4293-a26c-3485062e8e6a",
		CLI:        uxp.CLIClaude,
		ProjectCwd: "/home/dev/projects/usp",
		StartedAt:  time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC),
		EndedAt:    &end,
		TurnCount:  3,
	}
}

func sampleTurns() []session.Turn {
	return []session.Turn{
		{Role: session.RoleSystem, Content: "boot", Timestamp: time.Now()},
		{Role: session.RoleUser, Content: "fix the auth bug", Timestamp: time.Now()},
		{Role: session.RoleAssistant, Content: "let me look", Timestamp: time.Now(),
			ToolCalls: []session.ToolCall{
				{Name: "Bash", Input: "grep -r authMiddleware ."},
			}},
		{Role: session.RoleUser, Content: "and run the tests", Timestamp: time.Now()},
	}
}

func TestProject_SourceKeyAndHints(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{
		Agent:       "sami",
		LineageRoot: "11111111-1111-7111-8111-111111111111",
	})
	if got, want := p.SourceKey, "usp/fe2eb947-ecab-4293-a26c-3485062e8e6a"; got != want {
		t.Fatalf("source key: want %q, got %q", want, got)
	}
	want := []string{
		"#agent:sami",
		"#cli:claude",
		"#project:/home/dev/projects/usp",
		"#session:fe2eb947-ecab-4293-a26c-3485062e8e6a",
		"#lineage-root:11111111-1111-7111-8111-111111111111",
	}
	hs := p.HintsString()
	for _, w := range want {
		if !strings.Contains(hs, w) {
			t.Errorf("hints missing %q\nhints: %s", w, hs)
		}
	}
	if !strings.Contains(hs, "#hash:") {
		t.Errorf("expected #hash:<short> hint; got %s", hs)
	}
}

func TestProject_BodyContent(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{Agent: "sami"})
	wants := []string{
		"# Session fe2eb947-ecab-4293-a26c-3485062e8e6a",
		"- CLI: claude",
		"- Project: /home/dev/projects/usp",
		"- Started: 2026-04-25T12:00:00Z",
		"- Ended: 2026-04-25T12:30:00Z",
		"- Lineage root: self",
		"## Summary",
		"- fix the auth bug",
		"- and run the tests",
		"## Tool calls",
		"- Bash: grep -r authMiddleware .",
	}
	for _, w := range wants {
		if !strings.Contains(p.Body, w) {
			t.Errorf("body missing %q\nbody:\n%s", w, p.Body)
		}
	}
}

func TestProject_LineageRootSelf_NotEmittedAsHint(t *testing.T) {
	sess := sampleSession()
	p := Project(sess, sampleTurns(), ProjectOpts{
		Agent:       "sami",
		LineageRoot: sess.ID,
	})
	if strings.Contains(p.HintsString(), "#lineage-root:") {
		t.Errorf("self-root should not emit lineage-root hint; got %s", p.HintsString())
	}
}

func TestProject_Idempotency_StableHashAndKey(t *testing.T) {
	a := Project(sampleSession(), sampleTurns(), ProjectOpts{Agent: "sami"})
	b := Project(sampleSession(), sampleTurns(), ProjectOpts{Agent: "sami"})
	if a.SourceKey != b.SourceKey {
		t.Errorf("source key not stable: %s vs %s", a.SourceKey, b.SourceKey)
	}
	if a.ContentHash != b.ContentHash {
		t.Errorf("content hash not stable: %s vs %s", a.ContentHash, b.ContentHash)
	}
}

func TestProject_BodyTruncation(t *testing.T) {
	turns := []session.Turn{}
	for i := 0; i < 100; i++ {
		turns = append(turns, session.Turn{
			Role:    session.RoleUser,
			Content: strings.Repeat("a", 200),
		})
	}
	p := Project(sampleSession(), turns, ProjectOpts{
		Agent:        "sami",
		MaxBodyBytes: 512,
	})
	if len(p.Body) > 512+64 {
		t.Errorf("body should be truncated near 512 bytes; got %d", len(p.Body))
	}
	if !strings.Contains(p.Body, "(body truncated)") {
		t.Errorf("expected truncation marker; got tail: %q", p.Body[max(0, len(p.Body)-100):])
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestProject_NoTurns_StillProjects(t *testing.T) {
	p := Project(sampleSession(), nil, ProjectOpts{Agent: "sami"})
	if !strings.Contains(p.Body, "(no user prompts captured)") {
		t.Errorf("empty-turn body should note no prompts; got:\n%s", p.Body)
	}
	if !strings.Contains(p.Body, "(no tool calls)") {
		t.Errorf("empty-turn body should note no tool calls; got:\n%s", p.Body)
	}
}

func TestProject_OneLine_TrimsAndCollapses(t *testing.T) {
	turns := []session.Turn{
		{Role: session.RoleUser, Content: "line1\nline2\rline3   "},
	}
	p := Project(sampleSession(), turns, ProjectOpts{Agent: "sami"})
	if strings.Contains(p.Body, "line1\nline2") {
		t.Errorf("multiline user prompt should collapse; body:\n%s", p.Body)
	}
	if !strings.Contains(p.Body, "line1 line2 line3") {
		t.Errorf("collapsed line missing; body:\n%s", p.Body)
	}
}

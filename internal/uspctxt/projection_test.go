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

// TestProject_SourceKey: SourceKey retained as secondary dedup hint
// per spec §4.4 — primary identity is the @usp.session.<id> mention.
func TestProject_SourceKey(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{Agent: "sami"})
	if got, want := p.SourceKey, "usp/fe2eb947-ecab-4293-a26c-3485062e8e6a"; got != want {
		t.Fatalf("source key: want %q, got %q", want, got)
	}
}

// TestProject_Mentions_Order: spec §4.3 / §8.1 / mentions-registry.md.
// Order: @usp.session, @agent, @cli, @project, @usp.lineage, @scope.
func TestProject_Mentions_Order(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{
		Agent:       "sami",
		LineageRoot: "11111111-1111-7111-8111-111111111111",
		Scope:       "company",
	})
	want := []string{
		"@usp.session.fe2eb947-ecab-4293-a26c-3485062e8e6a",
		"@agent.sami",
		"@cli.claude",
		"@project.usp",
		"@usp.lineage.11111111-1111-7111-8111-111111111111",
		"@scope.company",
	}
	if len(p.Mentions) != len(want) {
		t.Fatalf("mention count: want %d, got %d (%v)", len(want), len(p.Mentions), p.Mentions)
	}
	for i, w := range want {
		if p.Mentions[i] != w {
			t.Errorf("mentions[%d]: want %q, got %q", i, w, p.Mentions[i])
		}
	}
	ms := p.MentionsString()
	if ms != strings.Join(want, " ") {
		t.Errorf("MentionsString: want %q, got %q", strings.Join(want, " "), ms)
	}
}

// TestProject_Mentions_MinimalSession: only @usp.session + @cli emitted
// when agent / project / lineage / scope all unset.
func TestProject_Mentions_MinimalSession(t *testing.T) {
	sess := sampleSession()
	sess.ProjectCwd = ""
	p := Project(sess, sampleTurns(), ProjectOpts{})
	want := []string{
		"@usp.session.fe2eb947-ecab-4293-a26c-3485062e8e6a",
		"@cli.claude",
	}
	if len(p.Mentions) != len(want) {
		t.Fatalf("minimal mentions: want %v, got %v", want, p.Mentions)
	}
	for i, w := range want {
		if p.Mentions[i] != w {
			t.Errorf("mentions[%d]: want %q, got %q", i, w, p.Mentions[i])
		}
	}
}

// TestProject_Mentions_LineageRootSelf: child-only sessions where root == self
// must NOT emit @usp.lineage (registry §"@usp.lineage" mint rule).
func TestProject_Mentions_LineageRootSelf(t *testing.T) {
	sess := sampleSession()
	p := Project(sess, sampleTurns(), ProjectOpts{
		Agent:       "sami",
		LineageRoot: sess.ID,
	})
	for _, m := range p.Mentions {
		if strings.HasPrefix(m, "@usp.lineage.") {
			t.Errorf("self-root must not emit lineage mention; got %v", p.Mentions)
		}
	}
}

// TestProject_Mentions_SlugNormalization: project basename uppercased + dotted
// must lowercase + dot→hyphen per registry §"@project.<slug>" mint rule.
func TestProject_Mentions_SlugNormalization(t *testing.T) {
	cases := []struct {
		cwd  string
		want string
	}{
		{"/Users/jad/.w/Idea-Crafters", "@project.idea-crafters"},
		{"/path/to/dot.ops", "@project.dot-ops"},
		{"/x/MyProject", "@project.myproject"},
		{"/y/.dotfiles", "@project.-dotfiles"},
		{"", ""}, // unset => no mention
	}
	for _, tc := range cases {
		sess := sampleSession()
		sess.ProjectCwd = tc.cwd
		p := Project(sess, sampleTurns(), ProjectOpts{Agent: "sami"})
		var got string
		for _, m := range p.Mentions {
			if strings.HasPrefix(m, "@project.") {
				got = m
			}
		}
		if got != tc.want {
			t.Errorf("cwd=%q: want %q, got %q (mentions=%v)", tc.cwd, tc.want, got, p.Mentions)
		}
	}
}

// TestProject_Mentions_SessionSlugLowercased: session UUIDs may arrive uppercase
// from upstream; mention slugs MUST be lowercased per mentions.md §Syntax.
func TestProject_Mentions_SessionSlugLowercased(t *testing.T) {
	sess := sampleSession()
	sess.ID = "FE2EB947-ECAB-4293-A26C-3485062E8E6A"
	p := Project(sess, sampleTurns(), ProjectOpts{Agent: "sami"})
	want := "@usp.session.fe2eb947-ecab-4293-a26c-3485062e8e6a"
	if p.Mentions[0] != want {
		t.Errorf("session mention: want %q, got %q", want, p.Mentions[0])
	}
}

// TestProject_Hints_OnlyHash: post-mentions, the only built-in hint is #hash.
// All identity tags moved to mentions per spec §8.2.
func TestProject_Hints_OnlyHash(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{
		Agent:       "sami",
		LineageRoot: "11111111-1111-7111-8111-111111111111",
		Scope:       "company",
	})
	if len(p.Hints) != 1 {
		t.Fatalf("hints: want exactly 1 (#hash:); got %d (%v)", len(p.Hints), p.Hints)
	}
	if !strings.HasPrefix(p.Hints[0], "#hash:") {
		t.Errorf("hints[0]: want #hash: prefix, got %q", p.Hints[0])
	}
	for _, banned := range []string{"#agent:", "#cli:", "#project:", "#session:", "#lineage-root:", "#scope:"} {
		if strings.Contains(p.HintsString(), banned) {
			t.Errorf("hints must not contain %q (moved to mentions); hints=%v", banned, p.Hints)
		}
	}
}

// TestProject_BodyMentionsLine: Mentions line embedded after heading per
// spec §4.3 — belt-and-suspenders for ctxt's inline parser.
func TestProject_BodyMentionsLine(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{
		Agent:       "sami",
		LineageRoot: "11111111-1111-7111-8111-111111111111",
	})
	if !strings.Contains(p.Body, "Mentions: @usp.session.") {
		t.Errorf("body missing 'Mentions:' line; body:\n%s", p.Body)
	}
	for _, want := range []string{
		"@usp.session.fe2eb947-ecab-4293-a26c-3485062e8e6a",
		"@agent.sami",
		"@cli.claude",
		"@project.usp",
		"@usp.lineage.11111111-1111-7111-8111-111111111111",
	} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("body Mentions line missing %q; body:\n%s", want, p.Body)
		}
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

// TestProject_Determinism: re-projecting identical input yields identical
// Mentions list (spec idempotency requirement).
func TestProject_Determinism(t *testing.T) {
	a := Project(sampleSession(), sampleTurns(), ProjectOpts{
		Agent: "sami", LineageRoot: "root-1", Scope: "company",
	})
	b := Project(sampleSession(), sampleTurns(), ProjectOpts{
		Agent: "sami", LineageRoot: "root-1", Scope: "company",
	})
	if a.SourceKey != b.SourceKey {
		t.Errorf("source key not stable: %s vs %s", a.SourceKey, b.SourceKey)
	}
	if a.ContentHash != b.ContentHash {
		t.Errorf("content hash not stable: %s vs %s", a.ContentHash, b.ContentHash)
	}
	if a.MentionsString() != b.MentionsString() {
		t.Errorf("mentions not stable:\n  a=%v\n  b=%v", a.Mentions, b.Mentions)
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

// --- T-0081: telemetry-derived signals --------------------------------------

// TestProject_Hints_CostBand: boundary table for cost band thresholds.
// <0.10 → low, <1.0 → med, ≥1.0 → high.
func TestProject_Hints_CostBand(t *testing.T) {
	cases := []struct {
		cost float64
		want string
	}{
		{0.099, "#cost:low"},
		{0.10, "#cost:med"},
		{0.999, "#cost:med"},
		{1.0, "#cost:high"},
		{5.0, "#cost:high"},
	}
	for _, tc := range cases {
		sess := sampleSession()
		sess.Metadata = map[string]any{"usage.cost_usd": tc.cost}
		p := Project(sess, sampleTurns(), ProjectOpts{Agent: "sami"})
		var got string
		for _, h := range p.Hints {
			if strings.HasPrefix(h, "#cost:") {
				got = h
			}
		}
		if got != tc.want {
			t.Errorf("cost=%v: want %q, got %q (hints=%v)", tc.cost, tc.want, got, p.Hints)
		}
	}
}

// TestProject_Hints_TokenBucket: boundary table for token volume buckets.
// ≤10_000 → small, ≤100_000 → med, >100_000 → large.
func TestProject_Hints_TokenBucket(t *testing.T) {
	cases := []struct {
		in, out int
		want    string
	}{
		{5_000, 5_000, "#tokens:small"},   // 10_000 boundary
		{5_001, 5_000, "#tokens:med"},     // 10_001
		{50_000, 50_000, "#tokens:med"},   // 100_000 boundary
		{50_001, 50_000, "#tokens:large"}, // 100_001
	}
	for _, tc := range cases {
		sess := sampleSession()
		sess.Metadata = map[string]any{
			"usage.tokens.input":  tc.in,
			"usage.tokens.output": tc.out,
		}
		p := Project(sess, sampleTurns(), ProjectOpts{Agent: "sami"})
		var got string
		for _, h := range p.Hints {
			if strings.HasPrefix(h, "#tokens:") {
				got = h
			}
		}
		if got != tc.want {
			t.Errorf("tokens=%d+%d: want %q, got %q (hints=%v)",
				tc.in, tc.out, tc.want, got, p.Hints)
		}
	}
}

// TestProject_Hints_NoTelemetry_NoBands: cost / tokens hints absent when
// Metadata has neither key. Existing #hash hint preserved.
func TestProject_Hints_NoTelemetry_NoBands(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{Agent: "sami"})
	if len(p.Hints) != 1 {
		t.Fatalf("hints: want only #hash; got %v", p.Hints)
	}
	for _, h := range p.Hints {
		if strings.HasPrefix(h, "#cost:") || strings.HasPrefix(h, "#tokens:") {
			t.Errorf("unexpected telemetry hint: %q", h)
		}
	}
}

// TestProject_Hints_ZeroCost_Skipped: cost == 0 must not emit a band.
func TestProject_Hints_ZeroCost_Skipped(t *testing.T) {
	sess := sampleSession()
	sess.Metadata = map[string]any{"usage.cost_usd": 0.0}
	p := Project(sess, sampleTurns(), ProjectOpts{Agent: "sami"})
	for _, h := range p.Hints {
		if strings.HasPrefix(h, "#cost:") {
			t.Errorf("zero cost must not emit band; got %q", h)
		}
	}
}

// TestProject_Telemetry_Section_Present: when telemetry keys present,
// body has a Telemetry section listing model, tokens, duration, cost.
func TestProject_Telemetry_Section_Present(t *testing.T) {
	sess := sampleSession()
	sess.Metadata = map[string]any{
		"assistant.model":         "claude-opus-4-7",
		"usage.tokens.input":      1_000,
		"usage.tokens.output":     2_000,
		"performance.duration_ms": int64(1_500),
		"usage.cost_usd":          0.25,
	}
	p := Project(sess, sampleTurns(), ProjectOpts{Agent: "sami"})
	if !strings.Contains(p.Body, "## Telemetry") {
		t.Fatalf("body missing Telemetry section:\n%s", p.Body)
	}
	for _, want := range []string{
		"model:",
		"tokens:",
		"$",
		"claude-opus-4-7",
		"3000",
		"1.5s",
		"$0.25",
	} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("Telemetry section missing %q; body:\n%s", want, p.Body)
		}
	}
}

// TestProject_Telemetry_Section_Absent: no telemetry keys → no heading.
func TestProject_Telemetry_Section_Absent(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{Agent: "sami"})
	if strings.Contains(p.Body, "## Telemetry") {
		t.Errorf("Telemetry section must be omitted when no keys; body:\n%s", p.Body)
	}
}

// TestProject_Telemetry_DurationFormat: <1000ms → "<n>ms"; ≥1000ms → "<x.y>s".
func TestProject_Telemetry_DurationFormat(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{123, "123ms"},
		{999, "999ms"},
		{1000, "1.0s"},
		{1500, "1.5s"},
		{12_345, "12.3s"},
	}
	for _, tc := range cases {
		sess := sampleSession()
		sess.Metadata = map[string]any{"performance.duration_ms": tc.ms}
		p := Project(sess, sampleTurns(), ProjectOpts{Agent: "sami"})
		if !strings.Contains(p.Body, "duration: "+tc.want) {
			t.Errorf("duration=%dms: want %q; body:\n%s", tc.ms, tc.want, p.Body)
		}
	}
}

// TestProject_Mentions_ModelSlug: model id slug normalization.
// Lowercased; `:`, `.`, `/` collapse to `-`.
func TestProject_Mentions_ModelSlug(t *testing.T) {
	cases := []struct {
		model string
		want  string
	}{
		{"claude-opus-4-7", "@model.claude-opus-4-7"},
		{"gpt-5.3-codex", "@model.gpt-5-3-codex"},
		{"claude-3.5-sonnet", "@model.claude-3-5-sonnet"},
		{"openai:gpt-4o", "@model.openai-gpt-4o"},
		{"anthropic/claude-opus", "@model.anthropic-claude-opus"},
		{"Claude-Opus-4-7", "@model.claude-opus-4-7"},
		{"", ""},
	}
	for _, tc := range cases {
		sess := sampleSession()
		if tc.model != "" {
			sess.Metadata = map[string]any{"assistant.model": tc.model}
		}
		p := Project(sess, sampleTurns(), ProjectOpts{Agent: "sami"})
		var got string
		for _, m := range p.Mentions {
			if strings.HasPrefix(m, "@model.") {
				got = m
			}
		}
		if got != tc.want {
			t.Errorf("model=%q: want %q, got %q (mentions=%v)",
				tc.model, tc.want, got, p.Mentions)
		}
	}
}

// TestProject_Mentions_ModelOrder: @model appended after identity mentions.
func TestProject_Mentions_ModelOrder(t *testing.T) {
	sess := sampleSession()
	sess.Metadata = map[string]any{"assistant.model": "claude-opus-4-7"}
	p := Project(sess, sampleTurns(), ProjectOpts{
		Agent: "sami", Scope: "company",
	})
	last := p.Mentions[len(p.Mentions)-1]
	if last != "@model.claude-opus-4-7" {
		t.Errorf("@model must be last; got mentions=%v", p.Mentions)
	}
}

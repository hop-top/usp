package uspctxt

import (
	"strings"
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"

	// Register the claude mention extractor for fixtures using CLIClaude.
	_ "hop.top/usp/adapters/claude"
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

// turnsWithToolCalls returns turns referencing files via Edit/Read/Write/Grep.
func turnsWithToolCalls() []session.Turn {
	return []session.Turn{
		{Role: session.RoleUser, Content: "fix authn"},
		{Role: session.RoleAssistant, ToolCalls: []session.ToolCall{
			{Name: "Read", Input: `{"file_path":"a.go"}`},
			{Name: "Edit", Input: `{"file_path":"b.go","old_string":"x","new_string":"y"}`},
		}},
		{Role: session.RoleAssistant, ToolCalls: []session.ToolCall{
			{Name: "Write", Input: `{"file_path":"c.go","content":"z"}`},
			// duplicate read of a.go — must dedupe.
			{Name: "Read", Input: `{"file_path":"a.go"}`},
		}},
	}
}

// TestProject_FileMentions_AppearedAfterIdentity: @file mentions are
// appended after identity mentions in the canonical order; identity
// mentions remain first and unchanged.
func TestProject_FileMentions_AppearedAfterIdentity(t *testing.T) {
	p := Project(sampleSession(), turnsWithToolCalls(), ProjectOpts{Agent: "sami"})
	want := []string{
		"@usp.session.fe2eb947-ecab-4293-a26c-3485062e8e6a",
		"@agent.sami",
		"@cli.claude",
		"@project.usp",
	}
	for i, w := range want {
		if p.Mentions[i] != w {
			t.Fatalf("identity mentions[%d]: want %q, got %q (all=%v)",
				i, w, p.Mentions[i], p.Mentions)
		}
	}
	tail := p.Mentions[len(want):]
	for _, m := range tail {
		if !strings.HasPrefix(m, "@file.") {
			t.Errorf("trailing mention not @file.*: %q (all=%v)", m, p.Mentions)
		}
	}
	for _, want := range []string{"@file.a-go", "@file.b-go", "@file.c-go"} {
		var found bool
		for _, m := range tail {
			if m == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %q in file mentions; got %v", want, tail)
		}
	}
}

// TestProject_FileMentions_Dedup: same file referenced twice yields one
// mention.
func TestProject_FileMentions_Dedup(t *testing.T) {
	turns := []session.Turn{
		{Role: session.RoleAssistant, ToolCalls: []session.ToolCall{
			{Name: "Read", Input: `{"file_path":"dup.go"}`},
			{Name: "Read", Input: `{"file_path":"dup.go"}`},
			{Name: "Read", Input: `{"file_path":"dup.go"}`},
		}},
	}
	p := Project(sampleSession(), turns, ProjectOpts{Agent: "sami"})
	count := 0
	for _, m := range p.Mentions {
		if m == "@file.dup-go" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("dup file: want 1, got %d (mentions=%v)", count, p.Mentions)
	}
}

// TestProject_FileMentions_WritePriority: with cap=3, the single Edit
// must beat the larger pool of Reads.
func TestProject_FileMentions_WritePriority(t *testing.T) {
	turns := []session.Turn{
		{Role: session.RoleAssistant, ToolCalls: []session.ToolCall{
			{Name: "Read", Input: `{"file_path":"b.go"}`},
			{Name: "Read", Input: `{"file_path":"c.go"}`},
			{Name: "Read", Input: `{"file_path":"d.go"}`},
			{Name: "Read", Input: `{"file_path":"e.go"}`},
			{Name: "Read", Input: `{"file_path":"f.go"}`},
			{Name: "Edit", Input: `{"file_path":"a.go"}`},
		}},
	}
	p := Project(sampleSession(), turns, ProjectOpts{
		Agent: "sami", MaxFileMentions: 3,
	})
	var files []string
	for _, m := range p.Mentions {
		if strings.HasPrefix(m, "@file.") {
			files = append(files, m)
		}
	}
	if len(files) != 3 {
		t.Fatalf("cap=3 mismatch: got %d (%v)", len(files), files)
	}
	if files[0] != "@file.a-go" {
		t.Errorf("first @file: want @file.a-go (Edit wins), got %q (%v)",
			files[0], files)
	}
}

// TestProject_FileMentions_Cap: MaxFileMentions truncates excess.
func TestProject_FileMentions_Cap(t *testing.T) {
	var calls []session.ToolCall
	for i := 0; i < 10; i++ {
		calls = append(calls, session.ToolCall{
			Name:  "Read",
			Input: `{"file_path":"f` + string(rune('0'+i)) + `.go"}`,
		})
	}
	turns := []session.Turn{{Role: session.RoleAssistant, ToolCalls: calls}}
	p := Project(sampleSession(), turns, ProjectOpts{MaxFileMentions: 3})
	var files []string
	for _, m := range p.Mentions {
		if strings.HasPrefix(m, "@file.") {
			files = append(files, m)
		}
	}
	if len(files) != 3 {
		t.Errorf("cap=3: got %d files (%v)", len(files), files)
	}
}

// TestProject_FileMentions_DefaultCap: zero MaxFileMentions => default 50.
func TestProject_FileMentions_DefaultCap(t *testing.T) {
	var calls []session.ToolCall
	for i := 0; i < 60; i++ {
		calls = append(calls, session.ToolCall{
			Name: "Read",
			// Use distinct paths via index.
			Input: `{"file_path":"dir/f` + itoa(i) + `.go"}`,
		})
	}
	turns := []session.Turn{{Role: session.RoleAssistant, ToolCalls: calls}}
	p := Project(sampleSession(), turns, ProjectOpts{}) // MaxFileMentions=0
	var files []string
	for _, m := range p.Mentions {
		if strings.HasPrefix(m, "@file.") {
			files = append(files, m)
		}
	}
	if len(files) != 50 {
		t.Errorf("default cap: want 50, got %d", len(files))
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf []byte
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	return string(buf)
}

// TestProject_SlashCommand_RendersAsName: slash-command turn renders
// /<name> in the Summary instead of the raw <command-name> tag.
func TestProject_SlashCommand_RendersAsName(t *testing.T) {
	turns := []session.Turn{
		{Role: session.RoleUser, Content: "regular question"},
		{Role: session.RoleUser, Subtype: "slash-command",
			Content: "<command-name>plan</command-name><command-message>foo</command-message>"},
	}
	p := Project(sampleSession(), turns, ProjectOpts{Agent: "sami"})
	if !strings.Contains(p.Body, "- /plan\n") {
		t.Errorf("body missing '- /plan'; body:\n%s", p.Body)
	}
	if strings.Contains(p.Body, "<command-name>") {
		t.Errorf("body should not contain raw <command-name> tag; body:\n%s", p.Body)
	}
}

// TestProject_SlashCommand_FallbackTokenForm: when no <command-name> tag,
// fall back to first whitespace-bounded token starting with `/`.
func TestProject_SlashCommand_FallbackTokenForm(t *testing.T) {
	turns := []session.Turn{
		{Role: session.RoleUser, Subtype: "slash-command",
			Content: "/review the diff"},
	}
	p := Project(sampleSession(), turns, ProjectOpts{Agent: "sami"})
	if !strings.Contains(p.Body, "- /review\n") {
		t.Errorf("body missing '- /review'; body:\n%s", p.Body)
	}
}

// TestProject_IntentsSection: distinct slash commands listed in
// first-occurrence order, deduped.
func TestProject_IntentsSection(t *testing.T) {
	turns := []session.Turn{
		{Role: session.RoleUser, Subtype: "slash-command",
			Content: "<command-name>plan</command-name>"},
		{Role: session.RoleUser, Content: "regular"},
		{Role: session.RoleUser, Subtype: "slash-command",
			Content: "<command-name>review</command-name>"},
		{Role: session.RoleUser, Subtype: "slash-command",
			Content: "<command-name>plan</command-name>"},
	}
	p := Project(sampleSession(), turns, ProjectOpts{Agent: "sami"})
	if !strings.Contains(p.Body, "## Intents") {
		t.Fatalf("expected ## Intents section; body:\n%s", p.Body)
	}
	idxPlan := strings.Index(p.Body, "- /plan\n")
	idxReview := strings.Index(p.Body, "- /review\n")
	if idxPlan < 0 || idxReview < 0 {
		t.Fatalf("missing /plan or /review in Intents; body:\n%s", p.Body)
	}
	intents := p.Body[strings.Index(p.Body, "## Intents"):]
	if strings.Count(intents, "- /plan\n") != 1 {
		t.Errorf("plan dedup failed: %q", intents)
	}
	// /plan first occurrence precedes /review.
	if strings.Index(intents, "- /plan\n") > strings.Index(intents, "- /review\n") {
		t.Errorf("intents not in first-occurrence order; section:\n%s", intents)
	}
}

// TestProject_IntentsSection_OmittedWhenNone: no slash-command turns =>
// no Intents section.
func TestProject_IntentsSection_OmittedWhenNone(t *testing.T) {
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{Agent: "sami"})
	if strings.Contains(p.Body, "## Intents") {
		t.Errorf("Intents section should be absent; body:\n%s", p.Body)
	}
}

// TestProject_IDENotif_Skipped: ide-notif user turns excluded from Summary.
func TestProject_IDENotif_Skipped(t *testing.T) {
	turns := []session.Turn{
		{Role: session.RoleUser, Subtype: "ide-notif",
			Content: "diagnostics changed"},
		{Role: session.RoleUser, Content: "real question"},
	}
	p := Project(sampleSession(), turns, ProjectOpts{Agent: "sami"})
	if strings.Contains(p.Body, "diagnostics changed") {
		t.Errorf("ide-notif content leaked into body; body:\n%s", p.Body)
	}
	if !strings.Contains(p.Body, "- real question\n") {
		t.Errorf("regular prompt missing; body:\n%s", p.Body)
	}
}

// TestProject_IdentityMentionsOrder_NoRegression: identity mentions
// remain in canonical order even when @file mentions are appended.
func TestProject_IdentityMentionsOrder_NoRegression(t *testing.T) {
	p := Project(sampleSession(), turnsWithToolCalls(), ProjectOpts{
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
	for i, w := range want {
		if p.Mentions[i] != w {
			t.Errorf("identity[%d]: want %q, got %q (all=%v)",
				i, w, p.Mentions[i], p.Mentions)
		}
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

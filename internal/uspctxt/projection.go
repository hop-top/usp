// Package uspctxt projects USP sessions into ctxt-ready payloads.
//
// Pipeline B of the ingestion-retrieval-pipelines track. Implements
// the bridge contract specified at:
//
//	<labspace>/hop/docs/ingestion-retrieval/spec.md §4
//
// Identity model (post T-0066 / T-0068): mentions, not tags. The
// canonical session anchor is `@usp.session.<id>`; tags retain only
// emergent classifier signals (`#hash:` for content fingerprinting).
// `--source-key` is kept as a secondary dedup hint per spec §4.4.
//
// Two pure halves are kept here (projection + state) so the cmd
// layer can wire them to os/exec + filesystem without bringing
// either concern into the core. Tests live in projection_test.go
// and state_test.go; cmd-level integration is gated by env var.
package uspctxt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"hop.top/usp/session"
)

// Granularity controls per-session vs per-turn projection.
//
// v0.1 default = SessionGranularity (one ctxt object per session).
// TurnGranularity is reserved for v0.2 (deferred per spec §4.3).
type Granularity string

const (
	SessionGranularity Granularity = "session"
	TurnGranularity    Granularity = "turn"
)

// Projection is the ctxt-ready payload for a single session.
//
// Body: markdown text fed to ctxt via stdin / --file.
// Mentions: canonical identity refs (`@namespace.slug`) joined into
//   `ctxt analyze --mentions`. Primary identity per spec §4.4.
// Hints: classifier tags joined into `ctxt analyze --hints`. v0.1
//   only emits `#hash:<short>`; identity moved to mentions.
// SourceKey: external dedup hint (`--source-key`); secondary signal
//   for ctxt's external-id catalog. Format: "usp/<session-id>".
//
// ContentHash backs a fallback dedup path: callers can compare
// hashes to short-circuit re-ingest of unchanged sessions before
// invoking ctxt at all.
type Projection struct {
	SourceKey   string
	Mentions    []string
	Hints       []string
	Body        string
	ContentHash string
}

// ProjectOpts configures the projection step.
type ProjectOpts struct {
	// Granularity = SessionGranularity (v0.1).
	Granularity Granularity

	// Agent is the producing aps profile id; emitted as @agent.<id>.
	// Empty => agent mention omitted.
	Agent string

	// LineageRoot is the first session ID in a cross-CLI lineage.
	// Empty / equal to sess.ID => no @usp.lineage.<root> mention.
	LineageRoot string

	// Scope is logical routing scope; emitted as @scope.<value>.
	// Empty => scope mention omitted.
	Scope string

	// MaxSummaryTurns caps user prompts in the Summary section.
	// Zero => default 5.
	MaxSummaryTurns int

	// MaxToolCalls caps the Tool calls flat list. Zero => default 20.
	MaxToolCalls int

	// MaxBodyBytes truncates the rendered body. Zero => default 8192.
	MaxBodyBytes int

	// MaxFileMentions caps @file mentions emitted per session.
	// Excess silently dropped. Zero => default 50.
	MaxFileMentions int
}

// Project renders a Projection from a Session + its turns.
//
// Pure: no I/O, no clock reads. Caller (cmd layer) supplies
// session + turns from usp adapters and writes the result via
// os/exec.
func Project(sess session.Session, turns []session.Turn, opts ProjectOpts) Projection {
	if opts.MaxSummaryTurns <= 0 {
		opts.MaxSummaryTurns = 5
	}
	if opts.MaxToolCalls <= 0 {
		opts.MaxToolCalls = 20
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 8192
	}
	if opts.MaxFileMentions <= 0 {
		opts.MaxFileMentions = 50
	}

	mentions := buildMentions(sess, turns, opts)
	body := renderBody(sess, turns, opts, mentions)
	if len(body) > opts.MaxBodyBytes {
		body = body[:opts.MaxBodyBytes] + "\n\n_(body truncated)_\n"
	}
	hash := sha256Hex(body)

	hints := buildHints(sess, hash)
	return Projection{
		SourceKey:   "usp/" + sess.ID,
		Mentions:    mentions,
		Hints:       hints,
		Body:        body,
		ContentHash: hash,
	}
}

// HintsString joins hints with single spaces for ctxt --hints.
func (p Projection) HintsString() string {
	return strings.Join(p.Hints, " ")
}

// MentionsString joins mentions with single spaces for ctxt --mentions.
func (p Projection) MentionsString() string {
	return strings.Join(p.Mentions, " ")
}

// buildMentions emits the canonical identity refs in spec-prescribed
// order: @usp.session, @agent, @cli, @project, @usp.lineage, @scope,
// then per-session @file mentions, then @model. Slugs lowercased per
// mentions.md §Syntax. Anchor is @usp.session; other namespaces are
// conditional on inputs.
//
// @file mentions are capped at opts.MaxFileMentions. Files touched by
// write tools (Edit, Write, MultiEdit, NotebookEdit) sort before files
// only read/searched. @model is emitted when assistant.model present.
func buildMentions(sess session.Session, turns []session.Turn, opts ProjectOpts) []string {
	out := []string{}
	out = append(out, "@usp.session."+strings.ToLower(strings.TrimSpace(sess.ID)))
	if opts.Agent != "" {
		out = append(out, "@agent."+strings.ToLower(strings.TrimSpace(opts.Agent)))
	}
	if cli := strings.ToLower(strings.TrimSpace(string(sess.CLI))); cli != "" {
		out = append(out, "@cli."+cli)
	}
	if slug := projectSlug(sess.ProjectCwd); slug != "" {
		out = append(out, "@project."+slug)
	}
	root := strings.TrimSpace(opts.LineageRoot)
	if root != "" && root != sess.ID {
		out = append(out, "@usp.lineage."+strings.ToLower(root))
	}
	if scope := strings.ToLower(strings.TrimSpace(opts.Scope)); scope != "" {
		out = append(out, "@scope."+scope)
	}
	out = append(out, fileMentions(sess, turns, opts.MaxFileMentions)...)
	if slug := modelSlug(metaString(sess.Metadata, "assistant.model")); slug != "" {
		out = append(out, "@model."+slug)
	}
	return out
}

// writeTools is the set of tool names whose @file mentions get
// priority placement (before read-only refs) per spec §file mentions.
var writeTools = map[string]bool{
	"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true,
}

// fileMentions runs the per-CLI extractor over each tool call,
// buckets by write-vs-read, dedupes globally (first-seen wins), and
// caps at max. Order: writes (first-seen) then reads (first-seen).
func fileMentions(sess session.Session, turns []session.Turn, max int) []string {
	if max <= 0 {
		return nil
	}
	fn := session.GetMentionExtractor(sess.CLI)
	if fn == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var writes, reads []string
	for _, t := range turns {
		for _, c := range t.ToolCalls {
			ms := fn(c)
			for _, m := range ms {
				if m == "" {
					continue
				}
				if _, ok := seen[m]; ok {
					continue
				}
				seen[m] = struct{}{}
				if writeTools[c.Name] {
					writes = append(writes, m)
				} else {
					reads = append(reads, m)
				}
			}
		}
	}
	out := append(writes, reads...)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// modelSlug normalizes a model id into a `@model.<slug>` per mentions
// registry: lowercased; `:`, `.`, `/` → `-`. Empty input => empty.
func modelSlug(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	id = strings.ToLower(id)
	id = strings.ReplaceAll(id, ":", "-")
	id = strings.ReplaceAll(id, ".", "-")
	id = strings.ReplaceAll(id, "/", "-")
	return id
}

// projectSlug normalizes a cwd into a `@project.<slug>` per registry
// mint rule: basename, lowercased, `.` → `-`. Empty cwd => empty.
func projectSlug(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	base := filepath.Base(cwd)
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, ".", "-")
	return base
}

// buildHints emits classifier hints. #hash is always present;
// telemetry-derived bands (#cost, #tokens) added when Metadata
// supplies the source values. Identity tags moved to mentions
// per spec §8.2.
//
// Cost bands (usage.cost_usd, USD): <0.10 low, <1.0 med, >=1.0 high.
// Token buckets (input+output): <=10_000 small, <=100_000 med,
// >100_000 large. "small/med/large" disambiguates from cost bands.
func buildHints(sess session.Session, hash string) []string {
	out := []string{"#hash:" + hash[:16]}
	if cost, ok := metaFloat(sess.Metadata, "usage.cost_usd"); ok && cost > 0 {
		out = append(out, "#cost:"+costBand(cost))
	}
	in, okIn := metaInt(sess.Metadata, "usage.tokens.input")
	outTok, okOut := metaInt(sess.Metadata, "usage.tokens.output")
	if okIn || okOut {
		out = append(out, "#tokens:"+tokenBucket(in+outTok))
	}
	return out
}

func costBand(c float64) string {
	switch {
	case c < 0.10:
		return "low"
	case c < 1.0:
		return "med"
	default:
		return "high"
	}
}

func tokenBucket(n int) string {
	switch {
	case n <= 10_000:
		return "small"
	case n <= 100_000:
		return "med"
	default:
		return "large"
	}
}

// metaString reads a string-typed Metadata key. Missing or wrong-typed
// keys return "".
func metaString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

// metaInt reads an int-typed Metadata key. Returns (0, false) when
// missing or wrong-typed.
func metaInt(m map[string]any, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key].(int)
	if !ok {
		return 0, false
	}
	return v, true
}

// metaInt64 reads an int64-typed Metadata key.
func metaInt64(m map[string]any, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key].(int64)
	if !ok {
		return 0, false
	}
	return v, true
}

// metaFloat reads a float64-typed Metadata key.
func metaFloat(m map[string]any, key string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key].(float64)
	if !ok {
		return 0, false
	}
	return v, true
}

func renderBody(sess session.Session, turns []session.Turn, opts ProjectOpts, mentions []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Session %s\n\n", sess.ID)
	if len(mentions) > 0 {
		// Embedded for ctxt's inline mention parser; --mentions flag
		// remains the source of truth for determinism.
		fmt.Fprintf(&b, "Mentions: %s\n\n", strings.Join(mentions, " "))
	}
	fmt.Fprintf(&b, "- CLI: %s\n", sess.CLI)
	if sess.ProjectCwd != "" {
		fmt.Fprintf(&b, "- Project: %s\n", sess.ProjectCwd)
	}
	fmt.Fprintf(&b, "- Started: %s\n", isoOrEmpty(sess.StartedAt))
	if sess.EndedAt != nil {
		fmt.Fprintf(&b, "- Ended: %s\n", sess.EndedAt.UTC().Format(time.RFC3339))
	} else {
		b.WriteString("- Ended: active\n")
	}
	fmt.Fprintf(&b, "- Turns: %d\n", sess.TurnCount)
	if opts.LineageRoot != "" {
		fmt.Fprintf(&b, "- Lineage root: %s\n", opts.LineageRoot)
	} else {
		b.WriteString("- Lineage root: self\n")
	}

	prompts := userPrompts(turns, opts.MaxSummaryTurns)
	b.WriteString("\n## Summary\n\n")
	if len(prompts) == 0 {
		b.WriteString("_(no user prompts captured)_\n")
	}
	for _, p := range prompts {
		fmt.Fprintf(&b, "- %s\n", oneLine(p))
	}

	if intents := sessionIntents(turns); len(intents) > 0 {
		b.WriteString("\n## Intents\n\n")
		for _, n := range intents {
			fmt.Fprintf(&b, "- /%s\n", n)
		}
	}

	calls := flatToolCalls(turns, opts.MaxToolCalls)
	b.WriteString("\n## Tool calls\n\n")
	if len(calls) == 0 {
		b.WriteString("_(no tool calls)_\n")
	}
	for _, c := range calls {
		fmt.Fprintf(&b, "- %s: %s\n", c.Name, oneLine(c.Input))
	}

	if tel := renderTelemetry(sess); tel != "" {
		b.WriteString("\n## Telemetry\n\n")
		b.WriteString(tel)
	}
	return b.String()
}

// renderTelemetry emits a bullet list of telemetry signals from
// Session.Metadata. Returns "" when no telemetry keys present so
// the caller can omit the heading entirely.
func renderTelemetry(sess session.Session) string {
	model := metaString(sess.Metadata, "assistant.model")
	in, okIn := metaInt(sess.Metadata, "usage.tokens.input")
	outTok, okOut := metaInt(sess.Metadata, "usage.tokens.output")
	dur, okDur := metaInt64(sess.Metadata, "performance.duration_ms")
	cost, okCost := metaFloat(sess.Metadata, "usage.cost_usd")

	if model == "" && !okIn && !okOut && !okDur && !okCost {
		return ""
	}
	var b strings.Builder
	if model != "" {
		fmt.Fprintf(&b, "- model: %s\n", model)
	}
	if okIn || okOut {
		fmt.Fprintf(&b, "- tokens: %d\n", in+outTok)
	}
	if okDur {
		fmt.Fprintf(&b, "- duration: %s\n", formatDuration(dur))
	}
	if okCost {
		fmt.Fprintf(&b, "- cost: %s\n", formatCost(cost))
	}
	return b.String()
}

func formatDuration(ms int64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
	}
	return fmt.Sprintf("%dms", ms)
}

func formatCost(c float64) string {
	return fmt.Sprintf("$%.2f", c)
}

func userPrompts(turns []session.Turn, max int) []string {
	out := []string{}
	for _, t := range turns {
		if t.Role != session.RoleUser {
			continue
		}
		if t.Subtype == "ide-notif" {
			continue
		}
		var s string
		if t.Subtype == "slash-command" {
			if name := slashCommandName(t.Content); name != "" {
				s = "/" + name
			}
		} else {
			s = strings.TrimSpace(t.Content)
		}
		if s == "" {
			continue
		}
		out = append(out, s)
		if len(out) >= max {
			break
		}
	}
	return out
}

// slashCommandTagRE captures the inner name of <command-name>...</command-name>.
var slashCommandTagRE = regexp.MustCompile(
	`(?s)<command-name>(.*?)</command-name>`)

// slashCommandName extracts a slash-command name from turn content.
// Prefers <command-name>NAME</command-name>; falls back to the first
// whitespace-bounded token starting with `/`. Leading `/` stripped.
func slashCommandName(content string) string {
	if m := slashCommandTagRE.FindStringSubmatch(content); len(m) == 2 {
		name := strings.TrimSpace(m[1])
		name = strings.TrimPrefix(name, "/")
		return name
	}
	for _, tok := range strings.Fields(content) {
		if strings.HasPrefix(tok, "/") {
			return strings.TrimPrefix(tok, "/")
		}
	}
	return ""
}

// sessionIntents returns distinct slash-command names invoked across
// turns, ordered by first occurrence.
func sessionIntents(turns []session.Turn) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, t := range turns {
		if t.Role != session.RoleUser || t.Subtype != "slash-command" {
			continue
		}
		name := slashCommandName(t.Content)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func flatToolCalls(turns []session.Turn, max int) []session.ToolCall {
	out := []session.ToolCall{}
	for _, t := range turns {
		for _, c := range t.ToolCalls {
			out = append(out, c)
			if len(out) >= max {
				return out
			}
		}
	}
	// stable order: turns iterate in input order, no extra sort needed
	return out
}

// oneLine collapses multi-line strings into a single line, trimming
// to a soft cap so payloads stay greppable.
func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	const cap = 240
	if len(s) > cap {
		return s[:cap] + "…"
	}
	return s
}

func isoOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// SortByStarted sorts sessions ascending by StartedAt. Bridge runs
// rely on ascending order so the high-water-mark only advances past
// successfully ingested sessions.
func SortByStarted(ss []session.Session) {
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].StartedAt.Before(ss[j].StartedAt)
	})
}

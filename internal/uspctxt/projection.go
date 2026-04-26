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

	mentions := buildMentions(sess, opts)
	body := renderBody(sess, turns, opts, mentions)
	if len(body) > opts.MaxBodyBytes {
		body = body[:opts.MaxBodyBytes] + "\n\n_(body truncated)_\n"
	}
	hash := sha256Hex(body)

	hints := buildHints(hash)
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
// order: @usp.session, @agent, @cli, @project, @usp.lineage, @scope.
// Slugs lowercased per mentions.md §Syntax. Anchor is @usp.session;
// other namespaces are conditional on inputs being non-empty.
func buildMentions(sess session.Session, opts ProjectOpts) []string {
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
	return out
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

// buildHints retains only the content-hash classifier; identity tags
// have moved to mentions per spec §8.2.
func buildHints(hash string) []string {
	return []string{"#hash:" + hash[:16]}
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

	calls := flatToolCalls(turns, opts.MaxToolCalls)
	b.WriteString("\n## Tool calls\n\n")
	if len(calls) == 0 {
		b.WriteString("_(no tool calls)_\n")
	}
	for _, c := range calls {
		fmt.Fprintf(&b, "- %s: %s\n", c.Name, oneLine(c.Input))
	}
	return b.String()
}

func userPrompts(turns []session.Turn, max int) []string {
	out := []string{}
	for _, t := range turns {
		if t.Role != session.RoleUser {
			continue
		}
		s := strings.TrimSpace(t.Content)
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

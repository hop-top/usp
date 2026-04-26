// Package uspctxt projects USP sessions into ctxt-ready payloads.
//
// Pipeline B of the ingestion-retrieval-pipelines track. Implements
// the bridge contract specified at:
//
//	<labspace>/hop/docs/ingestion-retrieval/spec.md §4
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
// Hints: tag list joined into ctxt analyze --hints "<...>".
// SourceKey: external dedup key (ctxt analyze --source-key).
//   Format: "usp/<session-id>". ctxt upserts by source-key,
//   delivering idempotency required by spec §4.4.
//
// ContentHash backs a fallback dedup path: callers can compare
// hashes to short-circuit re-ingest of unchanged sessions before
// invoking ctxt at all.
type Projection struct {
	SourceKey   string
	Hints       []string
	Body        string
	ContentHash string
}

// ProjectOpts configures the projection step.
type ProjectOpts struct {
	// Granularity = SessionGranularity (v0.1).
	Granularity Granularity

	// Agent is the producing aps profile id; emitted as #agent:<id>.
	// Empty => agent tag omitted.
	Agent string

	// LineageRoot is the first session ID in a cross-CLI lineage.
	// Empty => session is its own root; #lineage-root tag omitted.
	LineageRoot string

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

	body := renderBody(sess, turns, opts)
	if len(body) > opts.MaxBodyBytes {
		body = body[:opts.MaxBodyBytes] + "\n\n_(body truncated)_\n"
	}
	hash := sha256Hex(body)

	hints := buildHints(sess, opts, hash)
	return Projection{
		SourceKey:   "usp/" + sess.ID,
		Hints:       hints,
		Body:        body,
		ContentHash: hash,
	}
}

// HintsString joins hints with single spaces for ctxt --hints.
func (p Projection) HintsString() string {
	return strings.Join(p.Hints, " ")
}

func buildHints(sess session.Session, opts ProjectOpts, hash string) []string {
	hints := []string{}
	if opts.Agent != "" {
		hints = append(hints, "#agent:"+opts.Agent)
	}
	if cli := strings.TrimSpace(string(sess.CLI)); cli != "" {
		hints = append(hints, "#cli:"+cli)
	}
	if cwd := strings.TrimSpace(sess.ProjectCwd); cwd != "" {
		hints = append(hints, "#project:"+cwd)
	}
	hints = append(hints, "#session:"+sess.ID)
	root := opts.LineageRoot
	if root != "" && root != sess.ID {
		hints = append(hints, "#lineage-root:"+root)
	}
	hints = append(hints, "#hash:"+hash[:16])
	return hints
}

func renderBody(sess session.Session, turns []session.Turn, opts ProjectOpts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Session %s\n\n", sess.ID)
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

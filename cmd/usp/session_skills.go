package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"hop.top/kit/output"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

// skillRow is the renderable projection of a session.SkillEvent.
// Table mode shows session_id (truncated), CLI, timestamp, skill
// name, trigger turn ID (truncated), trigger query (truncated),
// and outcome. JSON/YAML preserves the full envelope.
type skillRow struct {
	SessionID     string `table:"SESSION"      json:"session_id"`
	CLI           string `table:"CLI"          json:"cli"`
	Timestamp     string `table:"TS"           json:"ts"`
	SkillName     string `table:"SKILL"        json:"skill_name"`
	TriggerTurnID string `table:"TRIGGER"      json:"trigger_turn_id"`
	TriggerQuery  string `table:"QUERY"        json:"trigger_query"`
	Outcome       string `table:"OUTCOME"      json:"outcome"`
	Unsupported   bool   `table:"UNSUPPORTED"  json:"unsupported,omitempty"`
}

const (
	skillRowQueryMax = 80
	skillRowTurnMax  = 12
	skillRowSessMax  = 12
)

func sessionSkillsCmd() *cobra.Command {
	var (
		sessionID string
		cliFlag   string
		project   string
		name      string
		since     string
		until     string
		format    string
	)

	cmd := &cobra.Command{
		Use:   "skills",
		Short: "List skills invoked in matching sessions",
		Long: `List every skill invoked across matching sessions.

Skills are slash-command invocations and explicit Skill tool
calls. Each row carries the session, CLI, timestamp, skill name,
the user turn that triggered it, a truncated trigger query, and
the outcome (invoked / declined / errored).

Filters AND-combine: --session, --cli, --project, --name,
--since, --until.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			sinceT, err := sessionutil.ParseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}
			untilT, err := sessionutil.ParseSince(until)
			if err != nil {
				return fmt.Errorf("invalid --until: %w", err)
			}

			adapters := sessionutil.FilterAdapters(allAdapters(), cliFlag)
			if adapters == nil {
				return fmt.Errorf("unknown CLI %q", cliFlag)
			}

			events, err := collectSkillEvents(adapters, sessionID, project, sinceT, untilT)
			if err != nil {
				return err
			}
			if name != "" {
				events = filterSkillByName(events, name)
			}
			sort.Slice(events, func(i, j int) bool {
				return events[i].Timestamp.Before(events[j].Timestamp)
			})

			if format != output.Table {
				if events == nil {
					events = []session.SkillEvent{}
				}
				return output.Render(os.Stdout, output.Format(format), events)
			}

			rows := skillRowsFromEvents(events)
			if len(rows) == 0 {
				fmt.Fprintln(os.Stderr, "No skill invocations found.")
				return nil
			}
			return output.Render(os.Stdout, output.Format(format), rows)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "",
		"Restrict to a single session (full or prefix ID)")
	cmd.Flags().StringVar(&cliFlag, "cli", "",
		"Restrict to a specific CLI (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&project, "project", "",
		"Restrict to a project cwd")
	cmd.Flags().StringVar(&name, "name", "",
		"Restrict to a specific skill name (substring match)")
	cmd.Flags().StringVar(&since, "since", "",
		"Only show skills invoked since (e.g. 2026-04-01, 7d, 24h)")
	cmd.Flags().StringVar(&until, "until", "",
		"Only show skills invoked until (e.g. 2026-04-15, 1h)")
	cmd.Flags().StringVar(&format, "format", "table",
		"Output format (table, json, yaml)")
	return cmd
}

// collectSkillEvents fans out to every adapter that supports
// skill extraction, narrowing by session/project/time. Adapters
// that do not implement [session.SkillExtractor] emit a single
// row per matched session with Unsupported=true.
func collectSkillEvents(
	adapters map[string]session.SessionAdapter,
	sessionID, project string,
	since, until time.Time,
) ([]session.SkillEvent, error) {
	var out []session.SkillEvent

	// Resolve sessions to scan: either a single ID, or every
	// session reachable through ListSessions(project).
	type target struct {
		s   session.Session
		cli string
		a   session.SessionAdapter
	}
	var targets []target

	if sessionID != "" {
		order := adapterOrder(sessionID)
		opts := sessionutil.ResolveOpts{Project: project, Since: since}
		s, cli, a, err := sessionutil.ResolveSessionID(sessionID, adapters, order, opts)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target{s: *s, cli: cli, a: a})
	} else {
		for cli, a := range adapters {
			ss, err := a.ListSessions(project)
			if err != nil {
				continue
			}
			for _, s := range ss {
				targets = append(targets, target{s: s, cli: cli, a: a})
			}
		}
	}

	for _, t := range targets {
		if !since.IsZero() && t.s.StartedAt.Before(since) {
			continue
		}
		if !until.IsZero() && t.s.StartedAt.After(until) {
			continue
		}
		ext, ok := t.a.(session.SkillExtractor)
		if !ok {
			out = append(out, session.SkillEvent{
				SessionID:   t.s.ID,
				CLI:         t.cli,
				Timestamp:   t.s.StartedAt,
				Unsupported: true,
			})
			continue
		}
		evs, err := ext.ExtractSkills(t.s.ID)
		if err != nil {
			continue
		}
		for _, e := range evs {
			if !since.IsZero() && e.Timestamp.Before(since) {
				continue
			}
			if !until.IsZero() && e.Timestamp.After(until) {
				continue
			}
			out = append(out, e)
		}
	}
	return out, nil
}

func filterSkillByName(events []session.SkillEvent, name string) []session.SkillEvent {
	needle := strings.ToLower(name)
	var out []session.SkillEvent
	for _, e := range events {
		if e.Unsupported {
			continue
		}
		if strings.Contains(strings.ToLower(e.Name), needle) {
			out = append(out, e)
		}
	}
	return out
}

func skillRowsFromEvents(events []session.SkillEvent) []skillRow {
	rows := make([]skillRow, len(events))
	for i, e := range events {
		ts := ""
		if !e.Timestamp.IsZero() {
			ts = e.Timestamp.UTC().Format("2006-01-02 15:04:05")
		}
		rows[i] = skillRow{
			SessionID:     truncateID(e.SessionID, skillRowSessMax),
			CLI:           e.CLI,
			Timestamp:     ts,
			SkillName:     e.Name,
			TriggerTurnID: truncateID(e.TriggerTurnID, skillRowTurnMax),
			TriggerQuery:  truncate(e.TriggerQuery, skillRowQueryMax),
			Outcome:       string(e.Outcome),
			Unsupported:   e.Unsupported,
		}
	}
	return rows
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// renderSkillsForShow emits the skills payload inline beneath a
// session show output. JSON/YAML callers receive the structured
// payload via the showResult.Skills field; this helper handles the
// table view.
func renderSkillsForShow(format string, events []session.SkillEvent) error {
	rows := skillRowsFromEvents(events)
	if len(rows) == 0 {
		if format == output.Table {
			fmt.Fprintln(os.Stdout, "\nSkills: (none)")
		}
		return nil
	}
	if format == output.Table {
		fmt.Fprintln(os.Stdout, "\nSkills:")
	}
	return output.Render(os.Stdout, output.Format(format), rows)
}


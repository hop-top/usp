package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"hop.top/kit/output"
	"hop.top/usp/session"
)

// sessionRow is the table-renderable projection of a session.
type sessionRow struct {
	ID      string `table:"ID"      json:"id"`
	CLI     string `table:"CLI"     json:"cli"`
	Project string `table:"PROJECT" json:"project"`
	Started string `table:"STARTED" json:"started"`
	Turns   int    `table:"TURNS"   json:"turns"`
}

func sessionListCmd() *cobra.Command {
	var (
		project string
		tool     string
		since   string
		limit   int
		format  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sessions across all supported CLIs",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Empty project = all projects.
			adapters := filterAdapters(allAdapters(), tool)
			if adapters == nil {
				return fmt.Errorf("unknown CLI %q", tool)
			}

			sinceTime, err := parseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}

			all := collectSessions(adapters, project)
			all = filterSince(all, sinceTime)
			all = sortAndLimit(all, limit)

			if len(all) == 0 {
				fmt.Fprintln(os.Stderr, "No sessions found.")
				return nil
			}

			return output.Render(os.Stdout, format, toRows(all))
		},
	}

	cmd.Flags().StringVar(&project, "project", "",
		"Filter to project dir (default: all projects)")
	cmd.Flags().StringVar(&tool, "tool", "",
		"Filter to a specific CLI (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&since, "since", "",
		"Show sessions since date (e.g. 2026-04-01, 7d, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 20,
		"Maximum sessions to display")
	cmd.Flags().StringVar(&format, "format", "table",
		"Output format (table, json, yaml)")
	return cmd
}

func sessionSearchCmd() *cobra.Command {
	var (
		project string
		tool     string
		since   string
		limit   int
		format  string
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search session content across all CLIs",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			query := strings.ToLower(args[0])

			adapters := filterAdapters(allAdapters(), tool)
			if adapters == nil {
				return fmt.Errorf("unknown CLI %q", tool)
			}

			sinceTime, err := parseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}

			all := collectSessions(adapters, project)
			all = filterSince(all, sinceTime)

			// Filter by content match via StreamTurns.
			var matched []session.Session
			adapterMap := allAdapters()
			for _, s := range all {
				a, ok := adapterMap[string(s.CLI)]
				if !ok {
					continue
				}
				ch, err := a.StreamTurns(s.ID)
				if err != nil {
					continue
				}
				for turn := range ch {
					if strings.Contains(
						strings.ToLower(turn.Content), query,
					) {
						matched = append(matched, s)
						// Drain remaining turns.
						for range ch {
						}
						break
					}
				}
			}

			matched = sortAndLimit(matched, limit)

			if len(matched) == 0 {
				fmt.Fprintln(os.Stderr, "No matching sessions.")
				return nil
			}

			return output.Render(os.Stdout, format, toRows(matched))
		},
	}

	cmd.Flags().StringVar(&project, "project", "",
		"Filter to project dir (default: all projects)")
	cmd.Flags().StringVar(&tool, "tool", "",
		"Filter to a specific CLI")
	cmd.Flags().StringVar(&since, "since", "",
		"Search sessions since date (e.g. 2026-04-01, 7d, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 20,
		"Maximum results")
	cmd.Flags().StringVar(&format, "format", "table",
		"Output format (table, json, yaml)")
	return cmd
}

// --- shared helpers ---

func filterAdapters(
	all map[string]session.SessionAdapter, tool string,
) map[string]session.SessionAdapter {
	if tool == "" {
		return all
	}
	a, ok := all[tool]
	if !ok {
		return nil
	}
	return map[string]session.SessionAdapter{tool: a}
}

func collectSessions(
	adapters map[string]session.SessionAdapter, project string,
) []session.Session {
	var all []session.Session
	for _, a := range adapters {
		sessions, err := a.ListSessions(project)
		if err != nil {
			continue
		}
		all = append(all, sessions...)
	}
	return all
}

func filterSince(
	ss []session.Session, since time.Time,
) []session.Session {
	if since.IsZero() {
		return ss
	}
	filtered := ss[:0]
	for _, s := range ss {
		if !s.StartedAt.Before(since) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func sortAndLimit(ss []session.Session, limit int) []session.Session {
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].StartedAt.After(ss[j].StartedAt)
	})
	if limit > 0 && len(ss) > limit {
		ss = ss[:limit]
	}
	return ss
}

func toRows(ss []session.Session) []sessionRow {
	rows := make([]sessionRow, len(ss))
	for i, s := range ss {
		rows[i] = sessionRow{
			ID:      truncateID(s.ID, 12),
			CLI:     string(s.CLI),
			Project: s.ProjectCwd,
			Started: relativeTime(s.StartedAt),
			Turns:   s.TurnCount,
		}
	}
	return rows
}

// parseSince parses duration shorthand (7d, 24h, 30m) or date
// (2026-04-01). Returns zero time if input is empty.
func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	// Try duration shorthand: Nd, Nh, Nm.
	if len(s) >= 2 {
		unit := s[len(s)-1]
		val := s[:len(s)-1]
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			switch unit {
			case 'd':
				return time.Now().Add(-time.Duration(n) * 24 * time.Hour), nil
			case 'h':
				return time.Now().Add(-time.Duration(n) * time.Hour), nil
			case 'm':
				return time.Now().Add(-time.Duration(n) * time.Minute), nil
			}
		}
	}
	// Try date formats.
	for _, layout := range []string{
		"2006-01-02",
		"2006-01-02T15:04:05",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf(
		"expected duration (7d, 24h) or date (2026-04-01), got %q", s)
}

func truncateID(id string, max int) string {
	if len(id) <= max {
		return id
	}
	return id[:max] + "…"
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

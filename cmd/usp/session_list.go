package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"hop.top/kit/output"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

// sessionRow is the table-renderable projection of a session.
//
// Table view shows the truncated TypeID (the canonical user-facing
// handle). JSON view exposes both the canonical id and the native
// CLI id so existing scripts that grep for native UUIDs keep working.
type sessionRow struct {
	ID       string `table:"ID"      json:"-"`
	FullID   string `table:"-"       json:"id"`
	NativeID string `table:"-"       json:"native_id,omitempty"`
	CLI      string `table:"CLI"     json:"cli"`
	Project  string `table:"PROJECT" json:"project"`
	Started  string `table:"STARTED" json:"started"`
	Turns    int    `table:"TURNS"   json:"turns"`
}

func sessionListCmd() *cobra.Command {
	var (
		project string
		tool    string
		since   string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sessions across all supported CLIs",
		RunE: func(c *cobra.Command, _ []string) error {
			format := formatFromViper()
			// Honor config defaults when flags weren't explicitly set.
			if !c.Flags().Changed("tool") && tool == "" {
				tool = rootViper.GetString("default_tool")
			}
			if !c.Flags().Changed("limit") {
				if v := rootViper.GetInt("default_limit"); v > 0 {
					limit = v
				}
			}
			adapters := sessionutil.FilterAdapters(allAdapters(), tool)
			if adapters == nil {
				return fmt.Errorf("unknown CLI %q", tool)
			}

			sinceTime, err := sessionutil.ParseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}

			all := sessionutil.CollectSessions(adapters, project)
			all = sessionutil.FilterSince(all, sinceTime)
			all = sessionutil.SortAndLimit(all, limit)

			if len(all) == 0 {
				listEmptyResult = true
				defer emitHint("list")
				if format != output.Table {
					return output.Render(os.Stdout, format, []sessionRow{})
				}
				fmt.Fprintln(os.Stderr, "No sessions found.")
				return nil
			}
			listEmptyResult = false
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
	return cmd
}

func sessionSearchCmd() *cobra.Command {
	var (
		project string
		tool    string
		since   string
		limit   int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search session content across all CLIs",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			format := formatFromViper()
			query := strings.ToLower(args[0])

			if !c.Flags().Changed("tool") && tool == "" {
				tool = rootViper.GetString("default_tool")
			}
			if !c.Flags().Changed("limit") {
				if v := rootViper.GetInt("default_limit"); v > 0 {
					limit = v
				}
			}
			adapters := sessionutil.FilterAdapters(allAdapters(), tool)
			if adapters == nil {
				return fmt.Errorf("unknown CLI %q", tool)
			}

			sinceTime, err := sessionutil.ParseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}

			all := sessionutil.CollectSessions(adapters, project)
			all = sessionutil.FilterSince(all, sinceTime)

			var matched []session.Session
			adapterMap := allAdapters()
			for _, s := range all {
				a, ok := adapterMap[string(s.CLI)]
				if !ok {
					continue
				}
				// Adapter lookups go through the native id.
				ch, err := a.StreamTurns(s.NativeID)
				if err != nil {
					continue
				}
				for turn := range ch {
					if strings.Contains(
						strings.ToLower(turn.Content), query,
					) {
						matched = append(matched, s)
						for range ch {
						}
						break
					}
				}
			}

			matched = sessionutil.SortAndLimit(matched, limit)

			if len(matched) == 0 {
				if format != output.Table {
					return output.Render(os.Stdout, format, []sessionRow{})
				}
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
	return cmd
}

// --- display helpers (not shared — UI-specific) ---

func toRows(ss []session.Session) []sessionRow {
	rows := make([]sessionRow, len(ss))
	for i, s := range ss {
		rows[i] = sessionRow{
			ID:       truncateID(s.ID, 16),
			FullID:   s.ID,
			NativeID: s.NativeID,
			CLI:      string(s.CLI),
			Project:  s.ProjectCwd,
			Started:  relativeTime(s.StartedAt),
			Turns:    s.TurnCount,
		}
	}
	return rows
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

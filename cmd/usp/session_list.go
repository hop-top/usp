package main

import (
	"fmt"
	"os"
	"sort"
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
		cli     string
		limit   int
		format  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sessions across all supported CLIs",
		RunE: func(_ *cobra.Command, _ []string) error {
			if project == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getwd: %w", err)
				}
				project = cwd
			}

			adapters := allAdapters()

			// Filter to single CLI if requested.
			if cli != "" {
				a, ok := adapters[cli]
				if !ok {
					return fmt.Errorf("unknown CLI %q", cli)
				}
				adapters = map[string]session.SessionAdapter{cli: a}
			}

			var all []session.Session
			for _, a := range adapters {
				sessions, err := a.ListSessions(project)
				if err != nil {
					continue // best-effort
				}
				all = append(all, sessions...)
			}

			// Sort by StartedAt descending.
			sort.Slice(all, func(i, j int) bool {
				return all[i].StartedAt.After(all[j].StartedAt)
			})

			// Apply limit.
			if limit > 0 && len(all) > limit {
				all = all[:limit]
			}

			if len(all) == 0 {
				fmt.Fprintln(os.Stderr, "No sessions found.")
				return nil
			}

			rows := make([]sessionRow, len(all))
			for i, s := range all {
				rows[i] = sessionRow{
					ID:      truncateID(s.ID, 12),
					CLI:     string(s.CLI),
					Project: s.ProjectCwd,
					Started: relativeTime(s.StartedAt),
					Turns:   s.TurnCount,
				}
			}

			return output.Render(os.Stdout, format, rows)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project directory (default: cwd)")
	cmd.Flags().StringVar(&cli, "cli", "", "Filter to a specific CLI adapter")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum sessions to display")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json, yaml)")
	return cmd
}

// truncateID shortens long IDs for table display.
func truncateID(id string, max int) string {
	if len(id) <= max {
		return id
	}
	return id[:max] + "…"
}

// relativeTime returns a human-friendly relative timestamp.
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

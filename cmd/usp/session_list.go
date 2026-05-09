package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/output"
	"hop.top/usp/internal/api"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

// sessionRow is the table-renderable projection of a session.
//
// Table view shows the truncated TypeID (the canonical user-facing
// handle). JSON view exposes both the canonical id and the native
// CLI id so existing scripts that grep for native UUIDs keep working.
type sessionRow struct {
	Actions  string `table:"ACTIONS,priority=9" json:"actions,omitempty"`
	Project  string `table:"PROJECT,priority=8" json:"project"`
	ID       string `table:"ID,priority=7"      json:"id"`
	Source   string `table:"SOURCE,priority=6"  json:"source"`
	Started  string `table:"STARTED,priority=5" json:"started"`
	FullID   string `table:"-"                  json:"-"`
	NativeID string `table:"-"                  json:"native_id,omitempty"`
	Turns    int    `table:"-"                  json:"turns"`
}

func sessionListCmd() *cobra.Command {
	var (
		project = defaultListProject()
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
			projectDefaulted := !c.Flags().Changed("project")
			if projectDefaulted {
				if cwd, err := os.Getwd(); err == nil {
					project = cwd
				}
			}
			sinceTime, err := sessionutil.ParseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}

			svc, err := newAPIService()
			if err != nil {
				return err
			}
			defer svc.Close()

			var items []api.SessionListItem
			req := api.ListSessionsRequest{
				Project: project,
				Tool:    tool,
				Since:   sinceTime,
				Limit:   limit,
			}
			if err := runWithProgress(c.Context(), "sessions", "loading sessions", func() error {
				var err error
				items, err = listSessionItemsWithProjectFallback(
					c.Context(), svc, req, projectDefaulted)
				return err
			}); err != nil {
				return err
			}

			if len(items) == 0 {
				listEmptyResult = true
				defer emitHint("list")
				if format != output.Table {
					return output.Render(os.Stdout, format, []sessionRow{})
				}
				fmt.Fprintln(os.Stderr, "No sessions found.")
				return nil
			}
			listEmptyResult = false
			return output.Render(os.Stdout, format, toItemRows(items, format))
		},
	}

	cmd.Flags().StringVar(&project, "project", project,
		"Filter to project dir (default: current directory; falls back to all projects)")
	cmd.Flags().StringVar(&tool, "tool", "",
		"Filter to a specific CLI (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&since, "since", "",
		"Show sessions since date (e.g. 2026-04-01, 7d, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 20,
		"Maximum sessions to display")
	return cmd
}

type sessionLister interface {
	ListSessions(context.Context, api.ListSessionsRequest) ([]session.Session, error)
}

type sessionItemLister interface {
	ListSessionItems(context.Context, api.ListSessionsRequest) ([]api.SessionListItem, error)
}

func listSessionsWithProjectFallback(
	ctx context.Context,
	svc sessionLister,
	req api.ListSessionsRequest,
	projectDefaulted bool,
) ([]session.Session, error) {
	all, err := svc.ListSessions(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(all) > 0 || !projectDefaulted || req.Project == "" {
		return all, nil
	}
	req.Project = ""
	return svc.ListSessions(ctx, req)
}

func listSessionItemsWithProjectFallback(
	ctx context.Context,
	svc sessionItemLister,
	req api.ListSessionsRequest,
	projectDefaulted bool,
) ([]api.SessionListItem, error) {
	items, err := svc.ListSessionItems(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(items) > 0 || !projectDefaulted || req.Project == "" {
		return items, nil
	}
	req.Project = ""
	return svc.ListSessionItems(ctx, req)
}

func defaultListProject() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
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
			sinceTime, err := sessionutil.ParseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}

			svc, err := newAPIService()
			if err != nil {
				return err
			}
			defer svc.Close()

			var matched []session.Session
			if err := runWithProgress(c.Context(), "sessions", "searching sessions", func() error {
				var err error
				matched, err = svc.SearchSessions(
					c.Context(),
					api.SearchSessionsRequest{
						Project: project,
						Tool:    tool,
						Query:   query,
						Since:   sinceTime,
						Limit:   limit,
					})
				return err
			}); err != nil {
				return err
			}

			if len(matched) == 0 {
				if format != output.Table {
					return output.Render(os.Stdout, format, []sessionRow{})
				}
				fmt.Fprintln(os.Stderr, "No matching sessions.")
				return nil
			}

			return output.Render(os.Stdout, format, toRows(matched, format))
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

// toRows projects sessions for output. Table render gets human-friendly
// "5m ago"; JSON/YAML gets RFC3339 (spec §8.3 — machine output is ISO 8601).
func toRows(ss []session.Session, format output.Format) []sessionRow {
	items := make([]api.SessionListItem, len(ss))
	for i, s := range ss {
		items[i] = api.SessionListItem{Session: s}
	}
	return toItemRows(items, format)
}

func toItemRows(items []api.SessionListItem, format output.Format) []sessionRow {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.Session.ID
	}
	rows := make([]sessionRow, len(items))
	for i, item := range items {
		s := item.Session
		id := s.ID
		project := s.ProjectCwd
		started := s.StartedAt.UTC().Format(time.RFC3339)
		if format == output.Table {
			id = queryableID(s.ID, ids)
			project = projectName(s.ProjectCwd)
			started = relativeTime(s.StartedAt)
		}
		rows[i] = sessionRow{
			ID:       id,
			FullID:   s.ID,
			NativeID: s.NativeID,
			Source:   sourceLabel(s.CLI, format),
			Project:  project,
			Started:  started,
			Actions:  item.Actions,
			Turns:    s.TurnCount,
		}
	}
	return rows
}

func queryableID(id string, all []string) string {
	const min = 12
	if len(id) <= min {
		return id
	}
	for n := min; n <= len(id); n++ {
		prefix := id[:n]
		unique := true
		for _, other := range all {
			if other == id {
				continue
			}
			if strings.HasPrefix(other, prefix) {
				unique = false
				break
			}
		}
		if unique {
			return prefix
		}
	}
	return id
}

func sourceLabel(cliName any, format output.Format) string {
	name := fmt.Sprint(cliName)
	if format != output.Table || !stdoutIsTTY() {
		return name
	}
	switch name {
	case "claude":
		return "✳"
	case "codex":
		return "◇"
	case "gemini":
		return "✦"
	case "opencode":
		return "⌘"
	default:
		return name
	}
}

func projectName(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) {
		return path
	}
	return base
}

func stdoutIsTTY() bool {
	if rootViper.GetBool("no-color") {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		return false
	}
	return true
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

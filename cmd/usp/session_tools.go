package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
	"hop.top/kit/go/console/output"
	"hop.top/usp/internal/api"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

type toolRow struct {
	SessionID string `table:"SESSION"  json:"session_id"`
	CLI       string `table:"CLI"      json:"cli"`
	Timestamp string `table:"TS"       json:"ts"`
	Category  string `table:"CATEGORY" json:"category,omitempty"`
	Label     string `table:"TOOL"     json:"label"`
	Name      string `table:"NATIVE"   json:"name"`
	TurnID    string `table:"TURN"     json:"turn_id,omitempty"`
	Input     string `table:"INPUT"    json:"input,omitempty"`
	Output    string `table:"OUTPUT"   json:"output,omitempty"`
}

const (
	toolRowSessMax = 12
	toolRowTurnMax = 12
	toolRowTextMax = 72
)

func sessionToolsCmd() *cobra.Command {
	var (
		sessionID string
		cliFlag   string
		project   string
		name      string
		category  string
		since     string
		until     string
	)

	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List tool calls in matching sessions",
		Long: `List every tool call across matching sessions.

Tool calls are enriched with Kit's universal tool taxonomy when a
mapping exists. Filters AND-combine: --session, --cli, --project,
--name, --category, --since, --until.`,
		RunE: func(c *cobra.Command, _ []string) error {
			sinceT, err := sessionutil.ParseSince(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}
			untilT, err := sessionutil.ParseSince(until)
			if err != nil {
				return fmt.Errorf("invalid --until: %w", err)
			}

			svc, err := newAPIService()
			if err != nil {
				return err
			}
			defer svc.Close()

			var events []session.ToolEvent
			if err := runWithProgress(c.Context(), "tools", "loading tool events", func() error {
				var err error
				events, err = svc.ListToolEvents(
					c.Context(),
					api.ListToolEventsRequest{
						SessionID: sessionID,
						CLI:       cliFlag,
						Project:   project,
						Name:      name,
						Category:  category,
						Since:     sinceT,
						Until:     untilT,
					})
				return err
			}); err != nil {
				return err
			}

			format := formatFromViper()
			if format != output.Table {
				if events == nil {
					events = []session.ToolEvent{}
				}
				return output.Render(os.Stdout, format, events)
			}

			rows := toolRowsFromEvents(events)
			if len(rows) == 0 {
				fmt.Fprintln(os.Stderr, "No tool calls found.")
				return nil
			}
			return output.Render(os.Stdout, format, rows)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "",
		"Restrict to a single session (full or prefix ID)")
	addCLIFlag(cmd, &cliFlag,
		"Restrict to a specific CLI (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&project, "project", "",
		"Restrict to a project cwd")
	cmd.Flags().StringVar(&name, "name", "",
		"Restrict to a tool name or universal label substring")
	cmd.Flags().StringVar(&category, "category", "",
		"Restrict to a universal category (research, edit, exec, todo, task, browser)")
	cmd.Flags().StringVar(&since, "since", "",
		"Only show tool calls since (e.g. 2026-04-01, 7d, 24h)")
	cmd.Flags().StringVar(&until, "until", "",
		"Only show tool calls until (e.g. 2026-04-15, 1h)")
	cli.SetSideEffect(cmd, cli.SideEffectRead)
	cli.SetIdempotency(cmd, cli.IdempotencyYes)
	return cmd
}

func toolRowsFromEvents(events []session.ToolEvent) []toolRow {
	rows := make([]toolRow, len(events))
	for i, e := range events {
		ts := ""
		if !e.Timestamp.IsZero() {
			ts = e.Timestamp.UTC().Format("2006-01-02 15:04:05")
		}
		rows[i] = toolRow{
			SessionID: truncateID(e.SessionID, toolRowSessMax),
			CLI:       e.CLI,
			Timestamp: ts,
			Category:  e.Category,
			Label:     e.Label,
			Name:      e.Name,
			TurnID:    truncateID(e.TurnID, toolRowTurnMax),
			Input:     truncate(toolOneLine(e.Input), toolRowTextMax),
			Output:    truncate(toolOneLine(e.Output), toolRowTextMax),
		}
	}
	return rows
}

func toolOneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

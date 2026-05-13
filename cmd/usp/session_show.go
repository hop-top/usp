package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
	"hop.top/kit/go/console/output"
	"hop.top/kit/go/core/uxp"
	"hop.top/usp/internal/api"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

func allAdapters() map[string]session.SessionAdapter {
	return api.DefaultAdapters()
}

// adapterOrder returns adapter names to try, prioritised by ID format.
func adapterOrder(id string) []string {
	return api.AdapterOrder(id)
}

// showResult is the unified payload for session show across all formats.
// Table format renders the top-level metadata; JSON/YAML includes turns.
type showResult struct {
	ID        string               `table:"ID"      json:"id"`
	NativeID  string               `table:"-"       json:"native_id,omitempty"`
	CLI       string               `table:"CLI"     json:"cli"`
	Project   string               `table:"PROJECT" json:"project"`
	StartedAt string               `table:"STARTED" json:"started_at"`
	EndedAt   string               `table:"ENDED"   json:"ended_at"`
	TurnCount int                  `table:"TURNS"   json:"turn_count"`
	Turns     []showTurn           `table:"-"       json:"turns"`
	Skills    []session.SkillEvent `table:"-"       json:"skills,omitempty"`
}

type showTurn struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Timestamp string         `json:"timestamp"`
	ToolCalls []showToolCall `json:"tool_calls,omitempty"`
}

// showToolCall enriches session.ToolCall with the universal name and
// rendering label (from kit's ToolCapability taxonomy) so transcripts
// stay legible across CLIs without losing the native tool name.
type showToolCall struct {
	Name      string `json:"name"`
	Universal string `json:"universal,omitempty"`
	Label     string `json:"label"`
	Input     string `json:"input,omitempty"`
	Output    string `json:"output,omitempty"`
}

// enrichToolCalls maps native ToolCalls to showToolCalls, attaching
// the universal name + label from the taxonomy. CLIs without a
// known mapping (detection-only entries, unknown native names) fall
// back to "[native:foo]" labels.
func enrichToolCalls(cli uxp.CLIName, calls []session.ToolCall) []showToolCall {
	out := make([]showToolCall, len(calls))
	for i, c := range calls {
		out[i] = showToolCall{
			Name:      c.Name,
			Universal: session.UniversalToolName(cli, c.Name),
			Label:     session.UniversalToolLabel(cli, c.Name),
			Input:     c.Input,
			Output:    c.Output,
		}
	}
	return out
}

func sessionShowCmd() *cobra.Command {
	var (
		cliFlag    string
		project    string
		since      string
		showSkills bool
	)

	cmd := &cobra.Command{
		Use:   "show [<id>]",
		Short: "Display session metadata and turn summary",
		Long: `Display the full metadata and turn-by-turn transcript for a
single session.

The positional <id> accepts either the canonical TypeID, a unique
prefix, or the underlying native CLI session id. When omitted, an
interactive picker selects from sessions in the current project.

Use --cli to restrict the prefix resolver to one adapter, --project
or --since to narrow the picker pool, and --skills to embed skill
invocations alongside the transcript. Output honors the global
--format flag (table, json, yaml).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			var sinceT time.Time
			if since != "" {
				t, err := sessionutil.ParseSince(since)
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				sinceT = t
			}

			fmt := formatFromViper()
			svc, err := newAPIService()
			if err != nil {
				return err
			}
			defer svc.Close()

			if id == "" {
				pickerProject := project
				if pickerProject == "" {
					pickerProject = defaultListProject()
				}
				selected, err := promptSelectSessionID(c.Context(), svc,
					api.ListSessionsRequest{
						Project: pickerProject,
						CLI:     cliFlag,
						Since:   sinceT,
					},
					"Choose session to show",
				)
				if err != nil {
					return err
				}
				id = selected
			}

			var detail *api.SessionDetail
			if err := runWithProgress(c.Context(), "session", "loading session", func() error {
				var err error
				detail, err = svc.ShowSession(
					c.Context(),
					api.ShowSessionRequest{
						ID:            id,
						CLI:           cliFlag,
						Project:       project,
						Since:         sinceT,
						IncludeSkills: showSkills,
					})
				return err
			}); err != nil {
				if strings.Contains(err.Error(), "not found") {
					return exitNotFound(err)
				}
				return err
			}

			cliName := uxp.CLIName(detail.CLI)
			var turns []showTurn
			for _, turn := range detail.Turns {
				turns = append(turns, showTurn{
					Role:      string(turn.Role),
					Content:   turn.Content,
					Timestamp: timestampForFormat(turn.Timestamp, fmt),
					ToolCalls: enrichToolCalls(cliName, turn.ToolCalls),
				})
			}

			ended := "active"
			if detail.Session.EndedAt != nil {
				ended = timestampForFormat(*detail.Session.EndedAt, fmt)
			}

			res := showResult{
				ID:        detail.Session.ID,
				NativeID:  detail.Session.NativeID,
				CLI:       detail.CLI,
				Project:   detail.Session.ProjectCwd,
				StartedAt: timestampForFormat(detail.Session.StartedAt, fmt),
				EndedAt:   ended,
				TurnCount: detail.Session.TurnCount,
				Turns:     turns,
				Skills:    detail.Skills,
			}

			if err := output.Render(os.Stdout, fmt, res); err != nil {
				return err
			}
			if showSkills && fmt == output.Table {
				return renderSkillsForShow(fmt, detail.Skills)
			}
			return nil
		},
	}
	addCLIFlag(cmd, &cliFlag, "Restrict search to a specific CLI")
	cmd.Flags().StringVar(&project, "project", "",
		"Narrow prefix match to project dir")
	cmd.Flags().StringVar(&since, "since", "",
		"Narrow prefix match to sessions since (e.g. 7d, 2026-04-01)")
	cmd.Flags().BoolVar(&showSkills, "skills", false,
		"Embed skill invocations alongside the session detail")
	cli.SetSideEffect(cmd, cli.SideEffectRead)
	cli.SetIdempotency(cmd, cli.IdempotencyYes)
	return cmd
}

// timestampForFormat renders t per spec §8.3: human-friendly for tables,
// RFC3339 (ISO 8601 + tz) for JSON/YAML/etc.
func timestampForFormat(t time.Time, format output.Format) string {
	if t.IsZero() {
		return ""
	}
	if format == output.Table {
		return t.Format("2006-01-02 15:04:05")
	}
	return t.UTC().Format(time.RFC3339)
}

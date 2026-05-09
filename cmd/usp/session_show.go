package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/output"
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
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	Timestamp string             `json:"timestamp"`
	ToolCalls []session.ToolCall `json:"tool_calls,omitempty"`
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
		Args:  cobra.MaximumNArgs(1),
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
						Tool:    cliFlag,
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
						Tool:          cliFlag,
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

			var turns []showTurn
			for _, turn := range detail.Turns {
				turns = append(turns, showTurn{
					Role:      string(turn.Role),
					Content:   turn.Content,
					Timestamp: timestampForFormat(turn.Timestamp, fmt),
					ToolCalls: turn.ToolCalls,
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
	cmd.Flags().StringVar(&cliFlag, "tool", "", "Restrict search to a specific CLI")
	cmd.Flags().StringVar(&project, "project", "",
		"Narrow prefix match to project dir")
	cmd.Flags().StringVar(&since, "since", "",
		"Narrow prefix match to sessions since (e.g. 7d, 2026-04-01)")
	cmd.Flags().BoolVar(&showSkills, "skills", false,
		"Embed skill invocations alongside the session detail")
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

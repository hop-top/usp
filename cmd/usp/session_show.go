package main

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
	"hop.top/kit/output"
	"hop.top/kit/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/adapters/opencode"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

// ID-format regexes for adapter priority hinting.
var (
	reUUIDv4    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	reUUIDv7    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	reOpenCode  = regexp.MustCompile(`^ses_[a-z0-9]{26}$`)
)

func allAdapters() map[string]session.SessionAdapter {
	return map[string]session.SessionAdapter{
		uxp.CLIClaude:   claude.New(),
		uxp.CLICodex:    &codex.Adapter{},
		uxp.CLIOpenCode: opencode.New(),
		uxp.CLIGemini:   &gemini.Adapter{},
	}
}

// adapterOrder returns adapter names to try, prioritised by ID format.
func adapterOrder(id string) []string {
	switch {
	case reUUIDv4.MatchString(id):
		return []string{uxp.CLIClaude, uxp.CLICodex, uxp.CLIOpenCode, uxp.CLIGemini}
	case reUUIDv7.MatchString(id):
		return []string{uxp.CLICodex, uxp.CLIClaude, uxp.CLIOpenCode, uxp.CLIGemini}
	case reOpenCode.MatchString(id):
		return []string{uxp.CLIOpenCode, uxp.CLIClaude, uxp.CLICodex, uxp.CLIGemini}
	default:
		return []string{uxp.CLIClaude, uxp.CLICodex, uxp.CLIOpenCode, uxp.CLIGemini}
	}
}

// showResult is the unified payload for session show across all formats.
// Table format renders the top-level metadata; JSON/YAML includes turns.
type showResult struct {
	ID        string     `table:"ID"      json:"id"`
	CLI       string     `table:"CLI"     json:"cli"`
	Project   string     `table:"PROJECT" json:"project"`
	StartedAt string     `table:"STARTED" json:"started_at"`
	EndedAt   string     `table:"ENDED"   json:"ended_at"`
	TurnCount int        `table:"TURNS"   json:"turn_count"`
	Turns     []showTurn `table:"-"       json:"turns"`
}

type showTurn struct {
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	Timestamp string             `json:"timestamp"`
	ToolCalls []session.ToolCall `json:"tool_calls,omitempty"`
}

func sessionShowCmd() *cobra.Command {
	var (
		cliFlag string
		format  string
		project string
		since   string
	)

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Display session metadata and turn summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			adapters := allAdapters()
			adapters = sessionutil.FilterAdapters(adapters, cliFlag)
			if adapters == nil {
				return fmt.Errorf("unknown CLI %q", cliFlag)
			}

			var opts sessionutil.ResolveOpts
			opts.Project = project
			if since != "" {
				t, err := sessionutil.ParseSince(since)
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				opts.Since = t
			}

			sess, matchedCLI, a, err := sessionutil.ResolveSessionID(
				id, adapters, adapterOrder(id), opts)
			if err != nil {
				return err
			}
			ch, err := a.StreamTurns(sess.ID)
			var turns []showTurn
			if err == nil {
				for turn := range ch {
					turns = append(turns, showTurn{
						Role:      string(turn.Role),
						Content:   turn.Content,
						Timestamp: turn.Timestamp.Format("2006-01-02 15:04:05"),
						ToolCalls: turn.ToolCalls,
					})
				}
			}

			ended := "active"
			if sess.EndedAt != nil {
				ended = sess.EndedAt.Format("2006-01-02 15:04:05")
			}

			return output.Render(os.Stdout, output.Format(format), showResult{
				ID:        sess.ID,
				CLI:       matchedCLI,
				Project:   sess.ProjectCwd,
				StartedAt: sess.StartedAt.Format("2006-01-02 15:04:05"),
				EndedAt:   ended,
				TurnCount: sess.TurnCount,
				Turns:     turns,
			})
		},
	}
	cmd.Flags().StringVar(&cliFlag, "tool", "", "Restrict search to a specific CLI")
	cmd.Flags().StringVar(&format, "format", "table",
		"Output format (table, json, yaml)")
	cmd.Flags().StringVar(&project, "project", "",
		"Narrow prefix match to project dir")
	cmd.Flags().StringVar(&since, "since", "",
		"Narrow prefix match to sessions since (e.g. 7d, 2026-04-01)")
	return cmd
}

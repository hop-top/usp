package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"hop.top/kit/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/adapters/opencode"
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

func sessionShowCmd() *cobra.Command {
	var cliFlag string

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Display session metadata and turn summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]
			adapters := allAdapters()

			var names []string
			if cliFlag != "" {
				a, ok := adapters[cliFlag]
				if !ok {
					return fmt.Errorf("unknown CLI %q", cliFlag)
				}
				adapters = map[string]session.SessionAdapter{cliFlag: a}
				names = []string{cliFlag}
			} else {
				names = adapterOrder(id)
			}

			var sess *session.Session
			var matchedCLI string
			for _, name := range names {
				a, ok := adapters[name]
				if !ok {
					continue
				}
				s, err := a.GetSession(id)
				if err != nil || s == nil {
					continue
				}
				sess = s
				matchedCLI = name
				break
			}
			if sess == nil {
				return fmt.Errorf("session %q not found", id)
			}

			// Print metadata.
			ended := "active"
			if sess.EndedAt != nil {
				ended = sess.EndedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(os.Stdout,
				"Session: %s\nCLI:     %s\nProject: %s\nStarted: %s\nEnded:   %s\nTurns:   %d\n",
				sess.ID, matchedCLI, sess.ProjectCwd,
				sess.StartedAt.Format("2006-01-02 15:04:05"),
				ended, sess.TurnCount,
			)

			// Stream turns.
			a := adapters[matchedCLI]
			ch, err := a.StreamTurns(id)
			if err != nil {
				return nil // metadata printed; turns unavailable
			}

			idx := 0
			for turn := range ch {
				idx++
				ts := turn.Timestamp.Format("2006-01-02 15:04:05")
				fmt.Fprintf(os.Stdout, "\nTurn %d [%s] %s\n", idx, turn.Role, ts)

				content := truncate(turn.Content, 100)
				if content != "" {
					fmt.Fprintf(os.Stdout, "  %s\n", content)
				}

				if len(turn.ToolCalls) > 0 {
					names := make([]string, len(turn.ToolCalls))
					for i, tc := range turn.ToolCalls {
						names[i] = tc.Name
					}
					fmt.Fprintf(os.Stdout, "  Tool calls: %s\n", strings.Join(names, ", "))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cliFlag, "tool", "", "Restrict search to a specific CLI")
	return cmd
}

func truncate(s string, max int) string {
	// Collapse newlines for display.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

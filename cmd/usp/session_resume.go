package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
	"hop.top/usp/internal/api"
	"hop.top/usp/session"
)

func sessionResumeCmd() *cobra.Command {
	var (
		cliFlag string
		filter  string
	)

	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume this project's latest session in another CLI",
		Long: "Resume the current project's most recently closed session " +
			"in another CLI without needing its session ID. The source " +
			"defaults to the latest closed session for the working " +
			"directory; --filter narrows by content reference " +
			"(issue#N, pr#N, mr#N, commit:<sha>). Multiple matches " +
			"prompt on a TTY; non-TTY paths return an error. Hand-off " +
			"is via syscall.Exec (see usp resume).",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			if !cliFlagChanged(c) && cliFlag == "" {
				cliFlag = rootViper.GetString("default_cli")
			}

			svc, err := newAPIService()
			if err != nil {
				return err
			}
			defer svc.Close()

			var sourceID string
			if filter != "" {
				sourceID, err = selectFilteredSession(c, svc, cwd, filter)
			} else {
				sourceID, err = latestClosedSessionID(c, svc, cwd)
			}
			if err != nil {
				return err
			}

			var res *api.ResumeSessionResult
			if err := runWithProgress(c.Context(), "resume", "preparing resume", func() error {
				var err error
				res, err = svc.ResumeSession(
					c.Context(),
					api.ResumeSessionRequest{
						ID:         sourceID,
						TargetCLI:  cliFlag,
						ProjectCWD: cwd,
					})
				return err
			}); err != nil {
				return err
			}

			slog.Info("resuming session",
				"target", res.TargetCLI, "session", res.TargetNative)
			return execResumeResult(res)
		},
	}
	addCLIFlag(cmd, &cliFlag,
		"Target CLI to resume in (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&filter, "filter", "",
		"Find a source session by content reference (issue#31, pr#2, mr#32, commit:<sha>)")

	// §4 (Layer-A) conformance. Same shape as `usp resume`: interactive
	// (syscall.Exec hands the terminal to the target CLI) and not
	// idempotent (re-running mints a fresh resume per the planner).
	// Depth-2 leaf under `session`, so no kit/top-level-verb.
	cli.SetSideEffect(cmd, cli.SideEffectInteractive)
	cli.SetIdempotency(cmd, cli.IdempotencyNo)
	return cmd
}

func latestClosedSessionID(c *cobra.Command, svc *api.Service, cwd string) (string, error) {
	var sessions []session.Session
	if err := runWithProgress(c.Context(), "sessions", "loading project sessions", func() error {
		var err error
		sessions, err = svc.ListSessions(
			c.Context(),
			api.ListSessionsRequest{Project: cwd},
		)
		return err
	}); err != nil {
		return "", err
	}
	if len(sessions) == 0 {
		return "", fmt.Errorf("no sessions found for %s", cwd)
	}
	return mostRecentClosed(sessions).ID, nil
}

func mostRecentClosed(sessions []session.Session) session.Session {
	var best *session.Session
	for i := range sessions {
		s := &sessions[i]
		if s.EndedAt == nil {
			continue
		}
		if best == nil || s.EndedAt.After(*best.EndedAt) {
			best = s
		}
	}
	if best != nil {
		return *best
	}
	return sessions[0]
}

func selectFilteredSession(
	c *cobra.Command,
	svc *api.Service,
	cwd string,
	filter string,
) (string, error) {
	var items []api.SessionListItem
	if err := runWithProgress(c.Context(), "sessions", "searching project sessions", func() error {
		var err error
		items, err = svc.FindSessionItems(
			c.Context(),
			api.FindSessionItemsRequest{
				Project: cwd,
				Filter:  filter,
				Limit:   sessionPickerLimit,
			})
		return err
	}); err != nil {
		return "", err
	}
	switch len(items) {
	case 0:
		return "", fmt.Errorf("no sessions match %q in %s", filter, cwd)
	case 1:
		return items[0].Session.ID, nil
	default:
		if !stdinIsTTY() {
			return "", fmt.Errorf("%d sessions match %q; run in a TTY to choose", len(items), filter)
		}
		return promptSessionChoice(c.Context(), items, "Choose session to resume")
	}
}

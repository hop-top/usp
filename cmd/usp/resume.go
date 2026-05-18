package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"hop.top/usp/internal/api"
)

func resumeCmd() *cobra.Command {
	var cliFlag string

	cmd := &cobra.Command{
		Use:   "resume [<id>]",
		Short: "Continue a conversation from one CLI in another",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			id := ""
			if len(args) == 1 {
				id = args[0]
			}

			svc, err := newAPIService()
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			if id == "" {
				selected, err := promptSelectSessionID(c.Context(), svc,
					api.ListSessionsRequest{
						Project: cwd,
					},
					"Choose session to resume",
				)
				if err != nil {
					return err
				}
				id = selected
			}
			if !cliFlagChanged(c) && cliFlag == "" {
				cliFlag = rootViper.GetString("default_cli")
			}

			var res *api.ResumeSessionResult
			if err := runWithProgress(c.Context(), "resume", "preparing resume", func() error {
				var err error
				res, err = svc.ResumeSession(
					c.Context(),
					api.ResumeSessionRequest{
						ID:         id,
						TargetCLI:  cliFlag,
						ProjectCWD: cwd,
					})
				return err
			}); err != nil {
				return err
			}

			slog.Info("resuming session",
				"target", res.TargetCLI, "session", res.TargetNative)
			resumeLastUSPID = res.USPID
			emitHint("resume")

			// Hand off to target CLI.
			argv := res.Command
			bin, err := exec.LookPath(argv[0])
			if err != nil {
				return fmt.Errorf("find %s: %w", argv[0], err)
			}
			return syscall.Exec(bin, argv, os.Environ())
		},
	}

	addCLIFlag(cmd, &cliFlag,
		"Target CLI to resume in (claude, codex, gemini, opencode)")
	return cmd
}

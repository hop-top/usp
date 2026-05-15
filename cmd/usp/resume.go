package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
	"hop.top/usp/internal/api"
)

func resumeCmd() *cobra.Command {
	var cliFlag string

	cmd := &cobra.Command{
		Use:   "resume [<id>]",
		Short: "Continue a conversation from one CLI in another",
		Long: "Resume a previously recorded usp session inside a " +
			"different CLI. Looks up the session by ID (or prompts " +
			"interactively when omitted), resolves the target CLI " +
			"binary, and hands off via syscall.Exec — the target " +
			"process replaces usp in place, so the resumed CLI owns " +
			"the terminal from that point on.\n\n" +
			"Because the handoff is an exec, usp's policy and " +
			"observability middleware end at the boundary: cancel " +
			"and exit semantics belong to the target CLI.",
		Args: cobra.MaximumNArgs(1),
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
			defer svc.Close()

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
			return execResumeResult(res)
		},
	}

	addCLIFlag(cmd, &cliFlag,
		"Target CLI to resume in (claude, codex, gemini, opencode)")

	// §4 (Layer-A) conformance. `resume` is interactive (it hands the
	// terminal off via syscall.Exec to the target CLI) and not
	// idempotent: re-running mints a fresh resume command per the
	// upstream planner, even with the same session id. Depth-1 leaf,
	// so kit/top-level-verb is required.
	cli.SetSideEffect(cmd, cli.SideEffectInteractive)
	cli.SetIdempotency(cmd, cli.IdempotencyNo)
	cli.SetTopLevelVerb(cmd)
	return cmd
}

func execResumeResult(res *api.ResumeSessionResult) error {
	resumeLastUSPID = res.USPID
	emitHint("resume")

	argv := res.Command
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("find %s: %w", argv[0], err)
	}
	return syscall.Exec(bin, argv, os.Environ())
}

package main

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"hop.top/kit/xdg"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/lineage"
	"hop.top/usp/session"
)

func resumeCmd() *cobra.Command {
	var (
		toolFlag    string
		sessionFlag string
	)

	cmd := &cobra.Command{
		Use:   "resume [<id>]",
		Short: "Continue a conversation from one CLI in another",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			// Resolve source session id: positional wins; --session is
			// a deprecated alias kept for one release.
			id := ""
			switch {
			case len(args) == 1 && sessionFlag != "":
				return fmt.Errorf("use only one of <id> or --session")
			case len(args) == 1:
				id = args[0]
			case sessionFlag != "":
				slog.Warn("--session is deprecated; pass <id> as a positional arg")
				id = sessionFlag
			}

			adapters := allAdapters()

			// Find source session.
			var (
				sess      *session.Session
				sourceCLI string
			)

			if id != "" {
				// Look up by ID across adapters.
				for _, name := range adapterOrder(id) {
					a, ok := adapters[name]
					if !ok {
						continue
					}
					s, err := a.GetSession(id)
					if err != nil || s == nil {
						continue
					}
					sess = s
					sourceCLI = name
					break
				}
				if sess == nil {
					return fmt.Errorf("session %q not found", id)
				}
			} else {
				// Find most recent session for cwd.
				all := sessionutil.CollectSessions(adapters, cwd)
				all = sessionutil.SortAndLimit(all, 1)
				if len(all) == 0 {
					return fmt.Errorf("no sessions found for %s", cwd)
				}
				sess = &all[0]
				sourceCLI = string(sess.CLI)
			}

			if toolFlag == "" {
				return fmt.Errorf(
					"specify target CLI with --tool (source: %s)", sourceCLI,
				)
			}

			// Get target adapter; assert ResumeAdapter.
			targetAdapter, ok := adapters[toolFlag]
			if !ok {
				return fmt.Errorf("unknown CLI %q", toolFlag)
			}
			target, ok := targetAdapter.(session.ResumeAdapter)
			if !ok {
				return fmt.Errorf(
					"%q does not support resume (ResumeAdapter not implemented)",
					toolFlag,
				)
			}

			// Stream turns from source.
			sourceAdapter := adapters[sourceCLI]
			ch, err := sourceAdapter.StreamTurns(sess.ID)
			if err != nil {
				return fmt.Errorf("stream turns: %w", err)
			}
			var turns []session.Turn
			for t := range ch {
				turns = append(turns, t)
			}

			// Inject into target.
			nativeID, err := target.InjectSession(cwd, turns)
			if err != nil {
				return fmt.Errorf("inject session: %w", err)
			}

			// Record lineage.
			stateDir, err := xdg.StateDir("usp")
			if err != nil {
				return fmt.Errorf("state dir: %w", err)
			}
			if err := os.MkdirAll(stateDir, 0o750); err != nil {
				return fmt.Errorf("create state dir: %w", err)
			}
			dbPath := filepath.Join(stateDir, "sessions.db")
			store, err := lineage.Open(dbPath)
			if err != nil {
				return fmt.Errorf("lineage store: %w", err)
			}
			defer store.Close()

			uspID := generateID()
			if err := store.CreateSession(uspID, cwd); err != nil {
				return fmt.Errorf("create session: %w", err)
			}
			if err := store.AddSegment(
				uspID, sourceCLI, sess.ID, len(turns),
			); err != nil {
				return fmt.Errorf("add source segment: %w", err)
			}
			if err := store.AddSegment(
				uspID, toolFlag, nativeID, 0,
			); err != nil {
				return fmt.Errorf("add target segment: %w", err)
			}

			slog.Info("resuming session",
				"target", toolFlag, "session", nativeID)

			// Hand off to target CLI.
			argv := target.ResumeCmd(nativeID)
			bin, err := exec.LookPath(argv[0])
			if err != nil {
				return fmt.Errorf("find %s: %w", argv[0], err)
			}
			return syscall.Exec(bin, argv, os.Environ())
		},
	}

	cmd.Flags().StringVar(&toolFlag, "tool", "",
		"Target CLI to resume in (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&sessionFlag, "session", "",
		"Source session ID (deprecated: pass <id> as positional arg)")
	_ = cmd.Flags().MarkHidden("session")
	return cmd
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	)
}

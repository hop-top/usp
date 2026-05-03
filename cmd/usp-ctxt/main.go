// Command usp-ctxt is the batch bridge from usp sessions to ctxt
// objects (Pipeline B; ingestion-retrieval-pipelines track T-0057).
//
// It walks each detected CLI's session list since a high-water-mark,
// projects each session into a ctxt-ready payload, and upserts
// through the ctxt CLI (`ctxt analyze --source-key usp/<id>`).
// State persists at ~/.local/share/usp-ctxt/last_run.json so re-runs
// only ingest new sessions.
//
// Spec: <labspace>/hop/docs/ingestion-retrieval/spec.md §4
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
)

var version = "dev"

func main() {
	root := cli.New(cli.Config{
		Name:    "usp-ctxt",
		Version: version,
		Short:   "Bridge usp sessions into ctxt (Pipeline B)",
		Accent:  "#7C5CFF",
	})
	root.Cmd.AddCommand(syncCmd())
	if err := root.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}

func syncCmd() *cobra.Command {
	var (
		agent       string
		statePath   string
		ctxtServer  string
		dryRun      bool
		perTimeout  int
		toolFilter  string
		projectFlag string
		verboseFlag bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Walk usp sessions since last run and ingest into ctxt",
		RunE: func(c *cobra.Command, _ []string) error {
			path := statePath
			if path == "" {
				p, err := defaultStatePath()
				if err != nil {
					return err
				}
				path = p
			}
			a := pickAgent(agent, os.Getenv("USP_CTXT_AGENT"))
			return runSync(c.Context(), syncParams{
				agent:       a,
				statePath:   path,
				ctxtServer:  ctxtServer,
				dryRun:      dryRun,
				perTimeout:  perTimeout,
				toolFilter:  toolFilter,
				projectFlag: projectFlag,
				verbose:     verboseFlag,
			}, os.Stderr, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "",
		"Producer agent id (aps profile); also: $USP_CTXT_AGENT")
	cmd.Flags().StringVar(&statePath, "state", "",
		"State file path (default: ~/.local/share/usp-ctxt/last_run.json)")
	cmd.Flags().StringVar(&ctxtServer, "ctxt-server", "",
		"ctxt server URL (default: ctxt's own default)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Project + log; do not invoke ctxt or update state")
	cmd.Flags().IntVar(&perTimeout, "per-call-timeout", 30,
		"Per ctxt invocation timeout in seconds")
	cmd.Flags().StringVar(&toolFilter, "tool", "",
		"Restrict to one CLI (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&projectFlag, "project", "",
		"Restrict to a project cwd")
	cmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false,
		"Log per-session decisions to stderr")
	return cmd
}

func pickAgent(flag, env string) string {
	if flag != "" {
		return flag
	}
	if env != "" {
		return env
	}
	return ""
}

func defaultStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home: %w", err)
	}
	return home + "/.local/share/usp-ctxt/last_run.json", nil
}

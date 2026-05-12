package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"hop.top/kit/go/console/cli"
	"hop.top/usp/internal/mcp"
)

func mcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve USP read APIs over MCP stdio",
		Long: `Start a long-lived MCP (Model Context Protocol) server
over stdio that exposes usp's read-only session APIs to a
compatible host (e.g. an editor or agent runtime).

The process reads JSON-RPC requests on stdin and writes responses
on stdout. It blocks until stdin closes or its context is
cancelled, and is intentionally interactive: stream a transcript,
list sessions, or inspect tool calls without spawning a fresh
process per query.

This command never mutates state; the underlying API service is
opened read-only.`,
		RunE: func(c *cobra.Command, _ []string) error {
			svc, err := newAPIService()
			if err != nil {
				return err
			}
			defer svc.Close()

			if err := mcp.New(svc).Serve(
				c.Context(), os.Stdin, os.Stdout,
			); err != nil {
				return fmt.Errorf("mcp: %w", err)
			}
			return nil
		},
	}
	cli.SetSideEffect(cmd, cli.SideEffectInteractive)
	cli.SetIdempotency(cmd, cli.IdempotencyNo)
	cli.SetTopLevelVerb(cmd)
	return cmd
}

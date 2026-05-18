package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"hop.top/usp/internal/mcp"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve USP read APIs over MCP stdio",
		RunE: func(c *cobra.Command, _ []string) error {
			svc, err := newAPIService()
			if err != nil {
				return err
			}
			defer func() { _ = svc.Close() }()

			if err := mcp.New(svc).Serve(
				c.Context(), os.Stdin, os.Stdout,
			); err != nil {
				return fmt.Errorf("mcp: %w", err)
			}
			return nil
		},
	}
}

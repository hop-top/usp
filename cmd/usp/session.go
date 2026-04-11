package main

import (
	"hop.top/kit/cli"

	"github.com/spf13/cobra"
)

func sessionCmd(_ *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage cross-CLI sessions",
	}
	cmd.AddCommand(
		sessionListCmd(),
		sessionShowCmd(),
		sessionSearchCmd(),
	)
	return cmd
}

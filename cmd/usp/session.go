package main

import (
	"hop.top/kit/go/console/cli"

	"github.com/spf13/cobra"
)

func sessionCmd(_ *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage cross-CLI sessions",
	}
	cmd.AddCommand(
		sessionListCmd(),
		sessionResumeCmd(),
		sessionShowCmd(),
		sessionSearchCmd(),
		sessionLineageCmd(),
		sessionSkillsCmd(),
		sessionToolsCmd(),
	)
	return cmd
}

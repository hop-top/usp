package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd prints the binary version. Mirrors `usp --version` but
// is grouped under MANAGEMENT in --help-all so users can discover it
// alongside config, completion, etc.
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the usp version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "usp v%s\n", version)
			return err
		},
	}
}

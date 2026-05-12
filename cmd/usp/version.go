package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"hop.top/kit/go/console/cli"
)

// versionCmd prints the binary version. Mirrors `usp --version` but
// is grouped under MANAGEMENT in --help-all so users can discover it
// alongside config, completion, etc.
func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the usp version",
		Long: `Print the usp binary version string (the same value
reported by the persistent --version global) on stdout and exit 0.

Output format is "usp v<version>\n". The command takes no
arguments, performs no I/O against the index or any adapter
store, and is safe to run in any environment.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "usp v%s\n", version)
			return err
		},
	}
	cli.SetSideEffect(cmd, cli.SideEffectRead)
	cli.SetIdempotency(cmd, cli.IdempotencyYes)
	cli.SetTopLevelVerb(cmd)
	return cmd
}

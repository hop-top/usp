package main

import "github.com/spf13/cobra"

func addCLIFlag(cmd *cobra.Command, target *string, usage string) {
	cmd.Flags().StringVar(target, "cli", "", usage)
}

func cliFlagChanged(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("cli")
}

package main

import (
	"github.com/spf13/cobra"

	"hop.top/kit/go/core/upgrade"
)

// uspGitHubRepo is the upstream release source for self-upgrade.
const uspGitHubRepo = "hop-top/usp"

// newUpgradeChecker constructs the kit/upgrade Checker for usp.
func newUpgradeChecker() *upgrade.Checker {
	return upgrade.New(
		upgrade.WithBinary("usp", version),
		upgrade.WithGitHub(uspGitHubRepo),
	)
}

// upgradeCmd returns the MANAGEMENT-grouped `upgrade` subtree.
// Mirrors tlc's wiring: thin wrapper around upgrade.RunCLI so the
// human prompt + GitHub release flow is identical across kit-built
// CLIs.
func upgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Check for and install updates",
		Long:  "Check for a newer version of usp and optionally install it.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			auto, _ := cmd.Flags().GetBool("auto")
			quiet, _ := cmd.Flags().GetBool("quiet")
			return upgrade.RunCLI(cmd.Context(), newUpgradeChecker(),
				upgrade.CLIOptions{AutoUpgrade: auto, Quiet: quiet})
		},
	}
	cmd.Flags().Bool("auto", false, "Install without prompting")
	cmd.Flags().BoolP("quiet", "q", false, "Suppress output when already up to date")
	return cmd
}

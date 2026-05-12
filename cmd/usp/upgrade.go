package main

import (
	"github.com/spf13/cobra"

	"hop.top/kit/go/console/cli"
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
//
// The "quiet" knob is read off the persistent kit global
// (registered by cli.New as --quiet on the root). The leaf used to
// register its own --quiet/-q pair, which silently shadowed the
// kit global for `usp upgrade` only; that local registration has
// been dropped so the kit global wins — see TestUpgradeCmd_QuietIsGlobal.
func upgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Check for and install updates",
		Long: `Check GitHub releases for a newer usp build and, if one
exists, prompt to install it in place.

Re-running upgrade against an already up-to-date binary is a
no-op (the kit global --quiet suppresses the "already current"
notice). Pass --auto to skip the interactive prompt and install
unconditionally; the operation overwrites the current binary on
disk, so treat it as a write step in any reproducible runbook.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auto, _ := cmd.Flags().GetBool("auto")
			quiet := rootViper.GetBool("quiet")
			return upgrade.RunCLI(cmd.Context(), newUpgradeChecker(),
				upgrade.CLIOptions{AutoUpgrade: auto, Quiet: quiet})
		},
	}
	cmd.Flags().Bool("auto", false, "Install without prompting")
	cli.SetSideEffect(cmd, cli.SideEffectWriteLocal)
	cli.SetIdempotency(cmd, cli.IdempotencyNo)
	cli.SetTopLevelVerb(cmd)
	return cmd
}

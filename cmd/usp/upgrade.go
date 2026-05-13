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
//
// --auto / kit --confirm bridge: kit's policy middleware only gates
// destructive* side-effects. `usp upgrade` is annotated write-local
// (the binary swap is recoverable by reinstall), so kit's confirm
// matrix does NOT fire here. The interactive y/N prompt lives inside
// kit's upgrade.RunCLI and is gated by --auto, which therefore acts
// as the local bridge equivalent of --confirm=yes for this command.
// We keep --auto rather than wiring kit's --confirm into RunCLI
// because the RunCLI prompt also accepts a third "snooze" answer
// that --confirm cannot express (auto|yes|no|prompt). See
// docs/migration/kit-12fcc-confirm.md for the full matrix.
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
disk, so treat it as a write step in any reproducible runbook.

Note: --auto is the local bridge equivalent of kit's --confirm=yes
for this command. Because upgrade is annotated write-local (the
binary swap is reversible by reinstall), kit's destructive-confirm
gate never fires here; --auto controls the interactive prompt
inside upgrade.RunCLI. The "snooze" answer at the interactive
prompt has no --confirm equivalent.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auto, _ := cmd.Flags().GetBool("auto")
			quiet := rootViper.GetBool("quiet")
			return upgrade.RunCLI(cmd.Context(), newUpgradeChecker(),
				upgrade.CLIOptions{AutoUpgrade: auto, Quiet: quiet})
		},
	}
	cmd.Flags().Bool("auto", false,
		"Install without prompting (bridge: equivalent to --confirm=yes for this command)")
	cli.SetSideEffect(cmd, cli.SideEffectWriteLocal)
	cli.SetIdempotency(cmd, cli.IdempotencyNo)
	cli.SetTopLevelVerb(cmd)
	return cmd
}

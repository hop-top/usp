// Package main — confirm_policy_test.go pins down the contract
// between usp's leaf side-effect annotations and kit's --confirm
// policy middleware (kit/go/console/cli/policy_runE.go).
//
// Today usp ships ZERO destructive* leaves; the most write-heavy
// leaves (`usp setup`, `usp upgrade`, `usp alias delete`) are all
// annotated write-local, which kit's gateConfirm does NOT gate (only
// destructive|destructive-local|destructive-shared trip the matrix).
//
// These tests are sentinels:
//
//   - If somebody adds a destructive* annotation to a usp leaf without
//     also wiring a bridge to kit's --confirm policy, TestConfirmPolicy_
//     NoDestructiveLeaves blows up so the change is reviewed.
//   - If kit's alias subtree starts shipping `alias delete` as
//     destructive-local upstream, TestConfirmPolicy_AliasDeleteIsWriteLocal
//     fails so we can re-evaluate the recoverability argument.
//
// Both invariants are documented in docs/migration/kit-12fcc-confirm.md.
package main

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/alias"
	"hop.top/kit/go/console/cli"
)

// buildIntegrationRoot mirrors main()'s wiring closely enough for an
// in-process conformance walk. We construct a fresh cli.Root with
// DisableValidate so the strict gate doesn't reject the partial tree
// (cmd-group hooks are not bound), then mount every usp-owned leaf
// plus the kit-owned alias subtree.
func buildIntegrationRoot(t *testing.T) *cli.Root {
	t.Helper()
	root := cli.New(
		cli.Config{
			Name:            "usp",
			Version:         "test",
			Short:           "test root",
			DisableValidate: true,
		},
		statusOption(),
	)
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		mcpCmd(),
		versionCmd(),
		upgradeCmd(),
	)
	store := alias.NewStore(filepath.Join(t.TempDir(), "aliases.yaml"))
	root.Cmd.AddCommand(aliasCmd(root, store))
	return root
}

// walkLeaves yields every runnable leaf in the tree, skipping group
// nodes and the built-in help/completion subcommands.
func walkLeaves(c *cobra.Command, fn func(*cobra.Command)) {
	if c.Runnable() && len(c.Commands()) == 0 {
		fn(c)
		return
	}
	for _, sub := range c.Commands() {
		// Skip cobra-shipped help/completion leaves; they aren't usp
		// surface area and carry no side-effect annotation.
		if sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}
		walkLeaves(sub, fn)
	}
}

// TestConfirmPolicy_NoDestructiveLeaves asserts the invariant
// captured in docs/migration/kit-12fcc-confirm.md: usp has no
// destructive* leaves today. Any new addition needs an explicit
// review of whether a local --force/--yes bridge is required.
func TestConfirmPolicy_NoDestructiveLeaves(t *testing.T) {
	root := buildIntegrationRoot(t)

	var offenders []string
	walkLeaves(root.Cmd, func(leaf *cobra.Command) {
		se, ok := cli.GetSideEffect(leaf)
		if !ok {
			return
		}
		switch se {
		case cli.SideEffectDestructive,
			cli.SideEffectDestructiveLocal,
			cli.SideEffectDestructiveShared:
			offenders = append(offenders, leaf.CommandPath()+" ("+string(se)+")")
		}
	})
	if len(offenders) > 0 {
		t.Fatalf("usp ships no destructive* leaves today; found %v. "+
			"If this is intentional, wire a --confirm bridge per "+
			"docs/migration/kit-12fcc-confirm.md and update this test.",
			offenders)
	}
}

// TestConfirmPolicy_AliasDeleteIsWriteLocal pins the kit-owned
// `usp alias delete` leaf at write-local. Rationale: alias entries
// live in $XDG_CONFIG_HOME/usp/aliases.yaml and are trivially
// recoverable by re-running `usp alias add`, so the irreversible-
// loss threshold for destructive-local doesn't apply.
//
// If kit changes its mind upstream and reclassifies as
// destructive-local, this test fails and we need to either
// re-justify (and adjust the test) or absorb the cross-platform
// confirm prompt as a UX regression.
func TestConfirmPolicy_AliasDeleteIsWriteLocal(t *testing.T) {
	root := buildIntegrationRoot(t)

	var del *cobra.Command
	for _, top := range root.Cmd.Commands() {
		if top.Name() != "alias" {
			continue
		}
		for _, sub := range top.Commands() {
			if sub.Name() == "delete" {
				del = sub
				break
			}
		}
	}
	if del == nil {
		t.Fatal("alias delete leaf not registered (kit Root.AliasCmd surface drift?)")
	}

	se, ok := cli.GetSideEffect(del)
	if !ok {
		t.Fatal("alias delete: missing kit/side-effect annotation upstream")
	}
	if se != cli.SideEffectWriteLocal {
		t.Errorf("alias delete: kit/side-effect = %q, want %q "+
			"(recoverability: aliases.yaml is rewritable via `alias add`)",
			se, cli.SideEffectWriteLocal)
	}
}

// TestConfirmPolicy_WriteLocalLeavesNotGated documents the
// behavioural side of the write-local choice: the three write-local
// leaves (setup, upgrade, alias delete) do NOT carry a
// kit/destructive-token annotation either, so kit's policy
// middleware will not refuse them on --confirm=no / non-TTY paths.
//
// This is the contract that lets `usp setup` and `usp upgrade --auto`
// run unattended in CI without an explicit --confirm=yes.
func TestConfirmPolicy_WriteLocalLeavesNotGated(t *testing.T) {
	root := buildIntegrationRoot(t)

	writeLocalLeaves := []string{
		"usp setup",
		"usp upgrade",
		"usp alias delete",
	}
	got := map[string]cli.SideEffect{}
	walkLeaves(root.Cmd, func(leaf *cobra.Command) {
		if se, ok := cli.GetSideEffect(leaf); ok {
			got[leaf.CommandPath()] = se
		}
	})

	for _, path := range writeLocalLeaves {
		se, ok := got[path]
		if !ok {
			t.Errorf("expected leaf %q to be registered", path)
			continue
		}
		if se != cli.SideEffectWriteLocal {
			t.Errorf("%s: side-effect = %q, want %q "+
				"(kit's gateConfirm only fires on destructive*, so this "+
				"leaf would silently bypass --confirm=no if reclassified)",
				path, se, cli.SideEffectWriteLocal)
		}
	}
}

// TestUpgradeCmd_AutoFlagBridge asserts the --auto/--confirm bridge
// is documented at the leaf surface. Sentinel: if someone removes
// the bridge note from the Long help or the flag usage string, this
// test fires so the docs/migration/kit-12fcc-confirm.md cross-
// reference doesn't go stale.
func TestUpgradeCmd_AutoFlagBridge(t *testing.T) {
	cmd := upgradeCmd()

	auto := cmd.Flags().Lookup("auto")
	if auto == nil {
		t.Fatal("upgrade: --auto flag missing")
	}
	if want := "--confirm=yes"; !contains(auto.Usage, want) {
		t.Errorf("upgrade --auto usage = %q, want substring %q "+
			"(bridge note keeps usage discoverable without reading docs/)",
			auto.Usage, want)
	}
	if want := "--confirm"; !contains(cmd.Long, want) {
		t.Errorf("upgrade Long help missing reference to %q; "+
			"docs/migration/kit-12fcc-confirm.md relies on this anchor",
			want)
	}
}

// contains is a tiny strings.Contains alias kept package-local so
// the test file doesn't pull a fresh strings import for one call.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	if needle == "" {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

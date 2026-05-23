package main

import (
	"strings"
	"testing"

	"hop.top/kit/go/console/cli"
)

// TestUpgradeCmd_Registered verifies the upgrade subcommand exists,
// belongs to MANAGEMENT, and exposes --auto. --quiet is intentionally
// NOT a local flag here; it lives on the kit root as a persistent
// global (see TestUpgradeCmd_QuietIsGlobal).
func TestUpgradeCmd_Registered(t *testing.T) {
	cmd := upgradeCmd()
	if cmd.Use != "upgrade" {
		t.Errorf("Use=%q, want upgrade", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short must not be empty")
	}
	if cmd.Flags().Lookup("auto") == nil {
		t.Error("--auto flag not registered")
	}
}

// TestUpgradeCmd_QuietIsGlobal asserts that the leaf does NOT
// register a local --quiet flag, so the kit-root persistent global
// (cli.New → pf.Bool("quiet", ...)) wins for `usp upgrade` like it
// does for every other usp leaf.
//
// Regression guard: a prior version of this file shipped
// `cmd.Flags().BoolP("quiet", "q", false, ...)` which silently
// shadowed the kit global for this one command (cobra accepts the
// redeclaration but the local binding masks the persistent one).
func TestUpgradeCmd_QuietIsGlobal(t *testing.T) {
	cmd := upgradeCmd()
	if got := cmd.Flags().Lookup("quiet"); got != nil {
		t.Fatalf("upgrade should not register a local --quiet flag; "+
			"shadows kit's persistent global (got: %#v)", got)
	}
	if got := cmd.Flags().ShorthandLookup("q"); got != nil {
		t.Fatalf("upgrade should not register -q shorthand; got: %#v", got)
	}
}

// TestUpgradeCmd_HelpText sanity checks the rendered help mentions
// "newer" / "release" so users can recognise it as a self-upgrade.
func TestUpgradeCmd_HelpText(t *testing.T) {
	cmd := upgradeCmd()
	long := strings.ToLower(cmd.Long)
	if !strings.Contains(long, "newer") {
		t.Errorf("Long help missing release-check phrasing: %q", cmd.Long)
	}
}

// TestUpgradeCmd_GroupAssignment confirms the registry assigns
// upgrade to MANAGEMENT so it lands under --help-all alongside
// version/config/completion.
func TestUpgradeCmd_GroupAssignment(t *testing.T) {
	got, ok := commandGroups["upgrade"]
	if !ok {
		t.Fatal("commandGroups missing upgrade entry")
	}
	if got != "management" {
		t.Errorf("upgrade group=%q, want management", got)
	}
}

// TestUpgradeCmd_Annotations verifies the kit/side-effect,
// kit/idempotent, and kit/top-level-verb annotations are in place
// so the strict CLI validator accepts the leaf.
func TestUpgradeCmd_Annotations(t *testing.T) {
	cmd := upgradeCmd()
	if se, ok := cli.GetSideEffect(cmd); !ok || se != cli.SideEffectWriteLocal {
		t.Errorf("upgrade side-effect = (%q,%v), want (write-local,true)", se, ok)
	}
	if id, ok := cli.GetIdempotency(cmd); !ok || id != cli.IdempotencyNo {
		t.Errorf("upgrade idempotency = (%q,%v), want (no,true)", id, ok)
	}
	if !cli.IsTopLevelVerb(cmd) {
		t.Error("upgrade missing kit/top-level-verb annotation")
	}
}

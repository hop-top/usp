package main

import (
	"strings"
	"testing"
)

// TestUpgradeCmd_Registered verifies the upgrade subcommand exists,
// belongs to MANAGEMENT, and exposes --auto / --quiet flags. The
// actual release-fetch flow needs network so we don't exercise it.
func TestUpgradeCmd_Registered(t *testing.T) {
	cmd := upgradeCmd()
	if cmd.Use != "upgrade" {
		t.Errorf("Use=%q, want upgrade", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short must not be empty")
	}
	for _, name := range []string{"auto", "quiet"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("--%s flag not registered", name)
		}
	}
}

// TestUpgradeCmd_HelpText sanity checks the rendered help mentions
// "version" / "install" so users can recognise it as a self-upgrade.
func TestUpgradeCmd_HelpText(t *testing.T) {
	cmd := upgradeCmd()
	long := cmd.Long
	if !strings.Contains(strings.ToLower(long),
		"newer version") {
		t.Errorf("Long help missing release-check phrasing: %q", long)
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

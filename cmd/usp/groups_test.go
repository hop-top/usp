package main

import (
	"testing"

	"hop.top/kit/go/console/cli"
)

func TestRootGroupsRegistered(t *testing.T) {
	root := cli.New(cli.Config{
		Name: "usp", Version: "test",
		Help:            cli.HelpConfig{Groups: rootGroups()},
		DisableValidate: true,
	})
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		mcpCmd(),
		versionCmd(),
	)
	applyCommandGroups(root.Cmd)

	want := map[string]string{
		"session": "knowledge",
		"resume":  "lifecycle",
		"doctor":  "organize",
		"setup":   "organize",
		"mcp":     "management",
		"version": "management",
	}
	for _, c := range root.Cmd.Commands() {
		if exp, ok := want[c.Name()]; ok {
			if c.GroupID != exp {
				t.Errorf("cmd %q GroupID = %q, want %q",
					c.Name(), c.GroupID, exp)
			}
		}
	}
}

func TestHelpDoesNotErrorWithGroups(t *testing.T) {
	root := cli.New(cli.Config{
		Name: "usp", Version: "test",
		Help:            cli.HelpConfig{Groups: rootGroups()},
		DisableValidate: true,
	})
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		mcpCmd(),
		versionCmd(),
	)
	applyCommandGroups(root.Cmd)

	root.Cmd.SetArgs([]string{"--help"})
	root.Cmd.SilenceUsage = true
	if err := root.Cmd.Execute(); err != nil {
		t.Errorf("--help with groups errored: %v", err)
	}
}

// TestHelpAll_RevealsManagement asserts --help-all surfaces the
// MANAGEMENT group, including the version command (which lives there
// per the audit-prescribed group assignment). Spec §4.3.
func TestHelpAll_RevealsManagement(t *testing.T) {
	root := cli.New(cli.Config{
		Name: "usp", Version: "test",
		Help:            cli.HelpConfig{Groups: rootGroups()},
		DisableValidate: true,
	})
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		mcpCmd(),
		versionCmd(),
	)
	applyCommandGroups(root.Cmd)

	// Walk root commands directly: kit's --help-all hides nothing in the
	// management group post-applyGroupVisibility. Verify version exists
	// AND is grouped into "management" — the two together prove the
	// help-all path will render it under MANAGEMENT.
	var sawVersion bool
	for _, c := range root.Cmd.Commands() {
		if c.Name() == "version" {
			sawVersion = true
			if c.GroupID != "management" {
				t.Errorf("version GroupID=%q, want management", c.GroupID)
			}
		}
	}
	if !sawVersion {
		t.Fatal("version subcommand missing from root")
	}

	// Sanity: --help-all flag is registered by kit.
	if root.Cmd.Flags().Lookup("help-all") == nil {
		t.Error("kit must register --help-all on root")
	}
	if root.Cmd.Flags().Lookup("help-management") == nil {
		t.Error("kit must register --help-management for the MANAGEMENT group")
	}
}

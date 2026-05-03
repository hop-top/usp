package main

import (
	"testing"

	"hop.top/kit/go/console/cli"
)

func TestRootGroupsRegistered(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test"})
	for _, g := range rootGroups() {
		root.Cmd.AddGroup(g)
	}
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		versionCmd(),
	)
	applyCommandGroups(root.Cmd)

	want := map[string]string{
		"session": "knowledge",
		"resume":  "lifecycle",
		"doctor":  "organize",
		"setup":   "organize",
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
	root := cli.New(cli.Config{Name: "usp", Version: "test"})
	for _, g := range rootGroups() {
		root.Cmd.AddGroup(g)
	}
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		versionCmd(),
	)
	applyCommandGroups(root.Cmd)

	root.Cmd.SetArgs([]string{"--help"})
	root.Cmd.SilenceUsage = true
	if err := root.Cmd.Execute(); err != nil {
		t.Errorf("--help with groups errored: %v", err)
	}
}

package main

import (
	"testing"

	"hop.top/kit/go/console/cli"
)

// TestFormatGlobalInherited confirms a --format flag passed at the
// root level binds to rootViper and is observable via formatFromViper.
func TestFormatGlobalInherited(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test"})
	prevRoot := rootViper
	rootViper = root.Viper
	t.Cleanup(func() { rootViper = prevRoot })

	root.Cmd.AddCommand(sessionCmd(root))
	root.Cmd.SetArgs([]string{"--format", "json", "session", "list", "--help"})
	root.Cmd.SilenceUsage = true
	root.Cmd.SilenceErrors = true

	if err := root.Cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := formatFromViper(); got != "json" {
		t.Errorf("formatFromViper after --format json = %q, want json", got)
	}
}

func TestFormatChildFlagInherited(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test"})
	prevRoot := rootViper
	rootViper = root.Viper
	t.Cleanup(func() { rootViper = prevRoot })

	root.Cmd.AddCommand(sessionCmd(root))
	// Persistent flag means the child accepts --format too.
	root.Cmd.SetArgs([]string{"session", "list", "--format", "yaml", "--help"})
	root.Cmd.SilenceUsage = true
	root.Cmd.SilenceErrors = true

	if err := root.Cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := formatFromViper(); got != "yaml" {
		t.Errorf("formatFromViper after child --format yaml = %q, want yaml", got)
	}
}

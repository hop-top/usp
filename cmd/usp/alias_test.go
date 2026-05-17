package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/alias"
	"hop.top/kit/go/console/cli"
)

// TestAliasCmd_GroupAssignment confirms `alias` is registered under
// MANAGEMENT so --help-all picks it up.
func TestAliasCmd_GroupAssignment(t *testing.T) {
	got, ok := commandGroups["alias"]
	if !ok {
		t.Fatal("commandGroups missing alias entry")
	}
	if got != "management" {
		t.Errorf("alias group=%q, want management", got)
	}
}

// TestAliasCmd_Subcommands ensures the kit-built alias group exposes
// list/add/delete leaves under usp.
func TestAliasCmd_Subcommands(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test", DisableValidate: true})
	store := alias.NewStore(filepath.Join(t.TempDir(), "aliases.yaml"))
	cmd := aliasCmd(root, store)

	want := map[string]bool{"list": false, "add": false, "delete": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}

// TestAliasCmd_AddPersistsToStore drives `alias add` end-to-end and
// verifies the YAML store gains the entry.
func TestAliasCmd_AddPersistsToStore(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test", DisableValidate: true})
	// stub session so add's target validation has something to bind
	// against if kit ever validates targets at add time. (Not today.)
	root.Cmd.AddCommand(&cobra.Command{Use: "session"})

	path := filepath.Join(t.TempDir(), "aliases.yaml")
	store := alias.NewStore(path)
	root.Cmd.AddCommand(aliasCmd(root, store))

	var buf bytes.Buffer
	root.Cmd.SetOut(&buf)
	root.Cmd.SetArgs([]string{"alias", "add", "ll", "session", "list"})
	if err := root.Execute(t.Context()); err != nil {
		t.Fatalf("execute: %v", err)
	}

	got, ok := store.Get("ll")
	if !ok {
		t.Fatal("alias 'll' not stored")
	}
	if !strings.Contains(got, "session list") {
		t.Errorf("target=%q, want it to contain 'session list'", got)
	}
}

// TestAliasStorePath returns a YAML file beneath the configured XDG
// config dir and surfaces no error in normal conditions.
func TestAliasStorePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := aliasStorePath()
	if err != nil {
		t.Fatalf("aliasStorePath: %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join("usp", "aliases.yaml")) {
		t.Errorf("path=%q, want suffix usp/aliases.yaml", got)
	}
}

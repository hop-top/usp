package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestConfigPath_Default prints "<defaults>" when no config file exists.
// The resolver places a synthetic default rung at the bottom of the
// chain so `config path` always has something to print.
func TestConfigPath_Default(t *testing.T) {
	cmd := configCmd(viper.New())
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"path", "--from", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		// Empty rungs return errNoConfig (exit 1). With a synthetic
		// default the chain always has a true rung — so any error here
		// is a regression in the resolver.
		t.Fatalf("config path: %v (stderr=%q)", err, errOut.String())
	}
	if !strings.Contains(out.String(), "<defaults>") {
		t.Errorf("expected <defaults> in stdout, got %q", out.String())
	}
}

// TestConfigPaths_ChainOrder asserts the highest-precedence-first
// chain: project → user → system → defaults. Each path appears on
// its own line per kit's text format.
func TestConfigPaths_ChainOrder(t *testing.T) {
	cmd := configCmd(viper.New())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	tmp := t.TempDir()
	cmd.SetArgs([]string{"paths", "--from", tmp})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config paths: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected ≥3 chain rungs, got %d: %v", len(lines), lines)
	}
	// project rung must end with .usp.yaml
	if !strings.HasSuffix(lines[0], "/.usp.yaml") {
		t.Errorf("first rung not project: %q", lines[0])
	}
	// last rung must be the synthetic defaults marker
	if lines[len(lines)-1] != "<defaults>" {
		t.Errorf("last rung not <defaults>: %q", lines[len(lines)-1])
	}
}

// TestConfigCmd_HasSubcommands verifies the kit-shared subcommands
// were attached to the config parent.
func TestConfigCmd_HasSubcommands(t *testing.T) {
	cmd := configCmd(viper.New())
	want := map[string]bool{"path": false, "paths": false}
	for _, c := range cmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("config subcommand %q not registered", name)
		}
	}
}

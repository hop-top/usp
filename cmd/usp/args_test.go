package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"hop.top/kit/cli"
)

// stubRunE replaces RunE with a no-op so tests exercise only flag/arg parsing.
func stubRunE(cmd *cobra.Command) {
	cmd.RunE = func(_ *cobra.Command, _ []string) error { return nil }
}

func TestArgsSessionListFlags(t *testing.T) {
	cmd := sessionListCmd()
	stubRunE(cmd)

	// --format is inherited from the root persistent flag.
	for _, name := range []string{"project", "tool", "since", "limit"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}

	if v, _ := cmd.Flags().GetInt("limit"); v != 20 {
		t.Errorf("limit default = %d, want 20", v)
	}
	for _, name := range []string{"project", "tool", "since"} {
		if v, _ := cmd.Flags().GetString(name); v != "" {
			t.Errorf("%s default = %q, want empty", name, v)
		}
	}

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("--help errored: %v", err)
	}
}

func TestArgsSessionShowRequiresOneArg(t *testing.T) {
	cmd := sessionShowCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if cmd.Flags().Lookup("tool") == nil {
		t.Error("missing --tool flag")
	}
	// --format is inherited from root persistent flags.

	// 0 args.
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	} else if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("unexpected error: %v", err)
	}

	// 2 args.
	cmd.SetArgs([]string{"id1", "id2"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 2 args")
	}

	// 1 arg.
	cmd.SetArgs([]string{"abc-123"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("1 arg should succeed: %v", err)
	}
}

func TestArgsResumeFlags(t *testing.T) {
	cmd := resumeCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	for _, name := range []string{"session", "tool"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}
	// --session must be hidden (deprecated alias).
	if f := cmd.Flags().Lookup("session"); f != nil && !f.Hidden {
		t.Error("--session flag should be hidden (deprecated)")
	}

	// Positional id (non-existent): lookup runs first, error from session.
	cmd.SetArgs([]string{"fake-session-id"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error with fake session id")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestArgsResumeSessionFlagStillWorks(t *testing.T) {
	cmd := resumeCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// --session alias still wires through.
	cmd.SetArgs([]string{"--session", "fake-session-id"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error with fake session via --session alias")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestArgsResumeRejectsBothPositionalAndSessionFlag(t *testing.T) {
	cmd := resumeCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cmd.SetArgs([]string{"abc", "--session", "xyz"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both <id> and --session given")
	}
	if !strings.Contains(err.Error(), "use only one") {
		t.Errorf("expected 'use only one' error, got: %v", err)
	}
}

func TestArgsResumeRejectsTooManyArgs(t *testing.T) {
	cmd := resumeCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cmd.SetArgs([]string{"a", "b"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 2 positional args")
	}
}

func TestArgsDoctorFlags(t *testing.T) {
	cmd := doctorCmd()

	if cmd.Flags().Lookup("tool") == nil {
		t.Error("missing --tool flag")
	}

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("--help errored: %v", err)
	}
}

func TestArgsInstallAcceptsZeroOrOneArg(t *testing.T) {
	cmd := installCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("0 args should succeed: %v", err)
	}

	cmd.SetArgs([]string{"all"})
	if err := cmd.Execute(); err != nil {
		t.Errorf(`"all" should succeed: %v`, err)
	}

	cmd.SetArgs([]string{"claude"})
	if err := cmd.Execute(); err != nil {
		t.Errorf(`"claude" should succeed: %v`, err)
	}

	cmd.SetArgs([]string{"claude", "codex"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 2 args")
	}
}

func TestArgsSessionLineageRequiresOneArg(t *testing.T) {
	cmd := sessionLineageCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}

	cmd.SetArgs([]string{"sess-id"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("1 arg should succeed: %v", err)
	}
}

func TestArgsSessionSearchRequiresOneArg(t *testing.T) {
	cmd := sessionSearchCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	for _, name := range []string{"tool", "project", "since", "limit"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}

	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with 0 args")
	}

	cmd.SetArgs([]string{"auth middleware"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("1 arg should succeed: %v", err)
	}
}

func TestArgsCommandWiring(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test"})
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		installCmd(),
	)

	rootNames := map[string]bool{}
	for _, c := range root.Cmd.Commands() {
		rootNames[c.Name()] = true
	}
	for _, want := range []string{"session", "resume", "doctor", "install"} {
		if !rootNames[want] {
			t.Errorf("root missing subcommand %q", want)
		}
	}

	for _, c := range root.Cmd.Commands() {
		if c.Name() == "session" {
			sesNames := map[string]bool{}
			for _, sub := range c.Commands() {
				sesNames[sub.Name()] = true
			}
			for _, want := range []string{"list", "show", "search", "lineage"} {
				if !sesNames[want] {
					t.Errorf("session missing subcommand %q", want)
				}
			}
			return
		}
	}
	t.Fatal("session command not found")
}

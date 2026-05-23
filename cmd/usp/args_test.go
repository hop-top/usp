package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
)

// stubRunE replaces RunE with a no-op so tests exercise only flag/arg parsing.
func stubRunE(cmd *cobra.Command) {
	cmd.RunE = func(_ *cobra.Command, _ []string) error { return nil }
}

func TestArgsSessionListFlags(t *testing.T) {
	cmd := sessionListCmd()
	stubRunE(cmd)

	// --format is inherited from the root persistent flag.
	for _, name := range []string{"project", "cli", "since", "limit"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}

	if v, _ := cmd.Flags().GetInt("limit"); v != 20 {
		t.Errorf("limit default = %d, want 20", v)
	}
	if v, _ := cmd.Flags().GetString("project"); v == "" {
		t.Error("project default should be current working directory")
	}
	for _, name := range []string{"cli", "since"} {
		if v, _ := cmd.Flags().GetString(name); v != "" {
			t.Errorf("%s default = %q, want empty", name, v)
		}
	}

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("--help errored: %v", err)
	}
}

func TestArgsSessionShowAcceptsOptionalArg(t *testing.T) {
	cmd := sessionShowCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if cmd.Flags().Lookup("cli") == nil {
		t.Error("missing --cli flag")
	}
	// --format is inherited from root persistent flags.

	// 0 args opens the interactive picker in the real RunE path.
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("0 args should pass arg validation: %v", err)
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

	if cmd.Flags().Lookup("cli") == nil {
		t.Error("missing --cli flag")
	}

	cmd.SetArgs([]string{"fake-session-id"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error with fake session id")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
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

	if cmd.Flags().Lookup("cli") == nil {
		t.Error("missing --cli flag")
	}

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("--help errored: %v", err)
	}
}

func TestArgsSetupAcceptsZeroOrOneArg(t *testing.T) {
	cmd := setupCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("0 args should succeed: %v", err)
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

	for _, name := range []string{"cli", "project", "since", "limit"} {
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

func TestArgsSessionResumeFlags(t *testing.T) {
	cmd := sessionResumeCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	for _, name := range []string{"cli", "filter"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}
	if cmd.Flags().Lookup("tool") != nil {
		t.Error("--tool flag should not be registered")
	}

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("--help errored: %v", err)
	}

	cmd = sessionResumeCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"sess_abc"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected positional args to be rejected")
	}
}

func TestArgsSessionSkillsFlags(t *testing.T) {
	cmd := sessionSkillsCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	for _, name := range []string{"session", "cli", "project", "name", "since", "until"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}
	if cmd.Flags().Lookup("tool") != nil {
		t.Error("--tool flag should not be registered")
	}
	// --format is inherited from the root persistent flag.

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("--help errored: %v", err)
	}
}

func TestArgsSessionToolsFlags(t *testing.T) {
	cmd := sessionToolsCmd()
	stubRunE(cmd)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	for _, name := range []string{"session", "cli", "project", "name", "category", "since", "until"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}
	if cmd.Flags().Lookup("tool") != nil {
		t.Error("--tool flag should not be registered")
	}

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("--help errored: %v", err)
	}
}

func TestArgsSessionShowSkillsFlag(t *testing.T) {
	cmd := sessionShowCmd()
	if cmd.Flags().Lookup("skills") == nil {
		t.Error("missing --skills flag on session show")
	}
}

func TestArgsCommandWiring(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test", DisableValidate: true})
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		mcpCmd(),
	)

	rootNames := map[string]bool{}
	for _, c := range root.Cmd.Commands() {
		rootNames[c.Name()] = true
	}
	for _, want := range []string{"session", "resume", "doctor", "setup", "mcp"} {
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
			for _, want := range []string{"list", "resume", "show", "search", "lineage", "skills", "tools"} {
				if !sesNames[want] {
					t.Errorf("session missing subcommand %q", want)
				}
			}
			return
		}
	}
	t.Fatal("session command not found")
}

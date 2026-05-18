package session_test

import (
	"slices"
	"testing"

	kitclaude "hop.top/kit/go/core/uxp/invoke/adapters/claude"
	kitcodex "hop.top/kit/go/core/uxp/invoke/adapters/codex"
	kitgemini "hop.top/kit/go/core/uxp/invoke/adapters/gemini"
	kitopencode "hop.top/kit/go/core/uxp/invoke/adapters/opencode"
	"hop.top/usp/session"
)

func TestResumeCmdFor_Claude(t *testing.T) {
	t.Parallel()
	cmd := session.ResumeCmdFor(kitclaude.New(), "abc-123")
	if len(cmd) == 0 || cmd[0] != "claude" {
		t.Fatalf("expected claude binary first; got %v", cmd)
	}
	if !slices.Contains(cmd, "--resume") || !slices.Contains(cmd, "abc-123") {
		t.Errorf("expected --resume abc-123: %v", cmd)
	}
}

func TestResumeCmdFor_Gemini(t *testing.T) {
	t.Parallel()
	cmd := session.ResumeCmdFor(kitgemini.New(), "uuid-1")
	if cmd[0] != "gemini" {
		t.Fatalf("expected gemini; got %v", cmd)
	}
	if !slices.Contains(cmd, "--resume") {
		t.Errorf("expected --resume: %v", cmd)
	}
}

func TestResumeCmdFor_Codex(t *testing.T) {
	t.Parallel()
	cmd := session.ResumeCmdFor(kitcodex.New(), "ses-z")
	// Default resume on codex is interactive `codex resume <id>`,
	// not `codex exec resume <id>`. usp resume hands argv to a
	// human, so interactive is correct.
	if cmd[0] != "codex" {
		t.Fatalf("expected codex; got %v", cmd)
	}
	if cmd[1] != "resume" {
		t.Errorf("expected resume subcommand; got %v", cmd)
	}
	if slices.Contains(cmd, "exec") {
		t.Errorf("default usp resume should not include exec subcommand: %v", cmd)
	}
}

func TestResumeCmdFor_Opencode(t *testing.T) {
	t.Parallel()
	cmd := session.ResumeCmdFor(kitopencode.New(), "sess_42")
	want := []string{"opencode", "run", "--session", "sess_42"}
	if len(cmd) != len(want) {
		t.Fatalf("len = %d, want %d (got %v)", len(cmd), len(want), cmd)
	}
	for i := range want {
		if cmd[i] != want[i] {
			t.Errorf("cmd[%d] = %q, want %q (full: %v)", i, cmd[i], want[i], cmd)
		}
	}
}

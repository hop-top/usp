package main

import (
	"bytes"
	"strings"
	"testing"

	"hop.top/kit/go/console/cli"
)

// TestVersionCmd_Annotations pins kit conformance annotations on
// the depth-1 version leaf.
func TestVersionCmd_Annotations(t *testing.T) {
	cmd := versionCmd()
	if se, ok := cli.GetSideEffect(cmd); !ok || se != cli.SideEffectRead {
		t.Errorf("version side-effect = (%q,%v), want (read,true)", se, ok)
	}
	if id, ok := cli.GetIdempotency(cmd); !ok || id != cli.IdempotencyYes {
		t.Errorf("version idempotency = (%q,%v), want (yes,true)", id, ok)
	}
	if !cli.IsTopLevelVerb(cmd) {
		t.Error("version missing kit/top-level-verb annotation")
	}
	if cmd.Long == "" {
		t.Error("version missing Long help")
	}
}

func TestVersionCmd_Output(t *testing.T) {
	cmd := versionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "usp v") {
		t.Errorf("output = %q, want prefix 'usp v'", got)
	}
}

func TestVersionCmd_NoArgs(t *testing.T) {
	cmd := versionCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"extra"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with positional arg")
	}
}

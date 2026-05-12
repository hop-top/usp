package main

import (
	"bytes"
	"strings"
	"testing"

	"hop.top/kit/go/console/cli"
	"hop.top/kit/go/console/output"
)

// TestSetupCmd_Annotations pins kit conformance annotations on the
// depth-1 setup leaf (writes the index DB → write-local, not
// idempotent).
func TestSetupCmd_Annotations(t *testing.T) {
	cmd := setupCmd()
	if se, ok := cli.GetSideEffect(cmd); !ok || se != cli.SideEffectWriteLocal {
		t.Errorf("setup side-effect = (%q,%v), want (write-local,true)", se, ok)
	}
	if id, ok := cli.GetIdempotency(cmd); !ok || id != cli.IdempotencyNo {
		t.Errorf("setup idempotency = (%q,%v), want (no,true)", id, ok)
	}
	if !cli.IsTopLevelVerb(cmd) {
		t.Error("setup missing kit/top-level-verb annotation")
	}
	if cmd.Long == "" {
		t.Error("setup missing Long help")
	}
}

func TestInstallRowsTableTags(t *testing.T) {
	rows := []setupRow{
		{CLI: "claude", Version: "v1.2.3", Status: "✓", Sessions: "5"},
	}
	var buf bytes.Buffer
	if err := output.Render(&buf, output.Table, rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"CLI", "VERSION", "STATUS", "SESSIONS", "claude", "v1.2.3"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestInstallRowsYAMLRender(t *testing.T) {
	rows := []setupRow{
		{CLI: "codex", Version: "v0.1.0", Status: "✓", Sessions: "3"},
	}
	var buf bytes.Buffer
	if err := output.Render(&buf, output.YAML, rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "cli: codex") {
		t.Errorf("expected yaml `cli: codex`, got: %s", out)
	}
	if !strings.Contains(out, "version: v0.1.0") {
		t.Errorf("expected yaml `version: v0.1.0`, got: %s", out)
	}
}

func TestInstallRowsJSONRender(t *testing.T) {
	rows := []setupRow{
		{CLI: "gemini", Version: "v0.0.1", Status: "✓", Sessions: "0"},
	}
	var buf bytes.Buffer
	if err := output.Render(&buf, output.JSON, rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"cli": "gemini"`) {
		t.Errorf("expected json cli=gemini, got: %s", out)
	}
}

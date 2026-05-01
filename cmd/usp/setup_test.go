package main

import (
	"bytes"
	"strings"
	"testing"

	"hop.top/kit/output"
)

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

package main

import (
	"testing"

	"hop.top/kit/go/console/cli"
)

// TestMcpCmd_Annotations pins kit conformance annotations on the
// depth-1 mcp leaf. The stdio server is long-lived and session-
// bound, hence interactive + idempotent=no.
func TestMcpCmd_Annotations(t *testing.T) {
	cmd := mcpCmd()
	if cmd.Use != "mcp" {
		t.Errorf("Use=%q, want mcp", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short must not be empty")
	}
	if cmd.Long == "" {
		t.Error("mcp missing Long help")
	}
	if se, ok := cli.GetSideEffect(cmd); !ok || se != cli.SideEffectInteractive {
		t.Errorf("mcp side-effect = (%q,%v), want (interactive,true)", se, ok)
	}
	if id, ok := cli.GetIdempotency(cmd); !ok || id != cli.IdempotencyNo {
		t.Errorf("mcp idempotency = (%q,%v), want (no,true)", id, ok)
	}
	if !cli.IsTopLevelVerb(cmd) {
		t.Error("mcp missing kit/top-level-verb annotation")
	}
}

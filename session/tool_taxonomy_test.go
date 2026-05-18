package session_test

import (
	"strings"
	"testing"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

func TestToolCapabilityFor_KnownNatives(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cli     uxp.CLIName
		native  string
		wantUni string
	}{
		{uxp.CLIClaude, "Bash", "shell.exec"},
		{uxp.CLIClaude, "Read", "file.read"},
		{uxp.CLIClaude, "Edit", "file.edit"},
		{uxp.CLIClaude, "MultiEdit", "file.edit"},
		{uxp.CLIClaude, "Glob", "file.search"},
		{uxp.CLIClaude, "Grep", "file.search"},
		{uxp.CLIClaude, "WebSearch", "web.search"},
		{uxp.CLIClaude, "WebFetch", "web.fetch"},
		{uxp.CLIClaude, "TodoWrite", "todo.write"},
		{uxp.CLIClaude, "Task", "task.spawn"},
		{uxp.CLICodex, "exec_command", "shell.exec"},
		{uxp.CLICodex, "apply_patch", "file.edit"},
		{uxp.CLICodex, "update_plan", "plan.update"},
		{uxp.CLIGemini, "shell", "shell.exec"},
		{uxp.CLIGemini, "read_file", "file.read"},
		{uxp.CLIOpenCode, "bash", "shell.exec"},
		{uxp.CLIOpenCode, "read", "file.read"},
		{uxp.CLIOpenCode, "edit", "file.edit"},
	}
	for _, tc := range cases {
		c, ok := session.ToolCapabilityFor(tc.cli, tc.native)
		if !ok {
			t.Errorf("(%s, %q): not found", tc.cli, tc.native)
			continue
		}
		if c.Universal != tc.wantUni {
			t.Errorf("(%s, %q).Universal = %q, want %q",
				tc.cli, tc.native, c.Universal, tc.wantUni)
		}
	}
}

func TestToolCapabilityFor_UnknownCLI(t *testing.T) {
	t.Parallel()
	_, ok := session.ToolCapabilityFor(uxp.CLIName("amp"), "anything")
	if ok {
		t.Error("expected ok=false for detection-only CLI")
	}
}

func TestToolCapabilityFor_UnknownNative(t *testing.T) {
	t.Parallel()
	_, ok := session.ToolCapabilityFor(uxp.CLIClaude, "NoSuchTool")
	if ok {
		t.Error("expected ok=false for unknown native")
	}
}

func TestUniversalToolLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cli    uxp.CLIName
		native string
		want   string
	}{
		{uxp.CLIClaude, "Bash", "shell.exec (Bash)"},
		{uxp.CLICodex, "exec_command", "shell.exec (exec_command)"},
		{uxp.CLIClaude, "MysteryTool", "[native:MysteryTool]"},
		{uxp.CLIName("amp"), "Bash", "[native:Bash]"},
	}
	for _, tc := range cases {
		got := session.UniversalToolLabel(tc.cli, tc.native)
		if got != tc.want {
			t.Errorf("UniversalToolLabel(%s, %q) = %q, want %q",
				tc.cli, tc.native, got, tc.want)
		}
	}
}

func TestUniversalToolName(t *testing.T) {
	t.Parallel()
	if got := session.UniversalToolName(uxp.CLIClaude, "Bash"); got != "shell.exec" {
		t.Errorf("got %q, want shell.exec", got)
	}
	if got := session.UniversalToolName(uxp.CLIClaude, "Unknown"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestCategorizeUniversalTool(t *testing.T) {
	t.Parallel()
	cases := []struct {
		uni  string
		want string
	}{
		{"file.read", "research"},
		{"file.search", "research"},
		{"web.search", "research"},
		{"web.fetch", "research"},
		{"file.write", "edit"},
		{"file.edit", "edit"},
		{"shell.exec", "exec"},
		{"todo.write", "todo"},
		{"task.spawn", "task"},
		{"plan.update", "task"},
		{"browser.operate", "browser"},
		{"unknown", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := session.CategorizeUniversalTool(tc.uni)
		if got != tc.want {
			t.Errorf("CategorizeUniversalTool(%q) = %q, want %q",
				tc.uni, got, tc.want)
		}
	}
}

func TestUniversalToolLabelStableAcrossCalls(t *testing.T) {
	t.Parallel()
	// Same inputs → same string (no internal map iteration leak).
	a := session.UniversalToolLabel(uxp.CLIClaude, "Bash")
	b := session.UniversalToolLabel(uxp.CLIClaude, "Bash")
	if a != b {
		t.Errorf("unstable label: %q vs %q", a, b)
	}
	if !strings.Contains(a, "shell.exec") || !strings.Contains(a, "Bash") {
		t.Errorf("label missing expected parts: %q", a)
	}
}

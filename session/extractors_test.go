package session

import (
	"reflect"
	"testing"

	"hop.top/kit/go/core/uxp"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "hello world", "hello world"},
		{"color", "\x1b[31mred\x1b[0m", "red"},
		{"multi", "\x1b[1;32mbold green\x1b[0m end", "bold green end"},
		{"cursor", "line1\x1b[2K\x1b[1Aline2", "line1line2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripANSI(tt.in); got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalFileMention(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		path string
		want string
	}{
		{"empty path", "/proj", "", ""},
		{"rel simple", "", "session/envelope.go",
			"@file.session-envelope-go"},
		{"rel with dot prefix", "", "./adapters/claude.go",
			"@file.adapters-claude-go"},
		{"abs in cwd", "/proj", "/proj/cmd/main.go",
			"@file.cmd-main-go"},
		{"abs outside cwd", "/proj", "/other/x.go", ""},
		{"abs no cwd", "", "/abs/path.go", ""},
		{"uppercase normalized", "", "Session/Foo.GO",
			"@file.session-foo-go"},
		{"nested", "", "a/b/c/d.txt",
			"@file.a-b-c-d-txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanonicalFileMention(tt.cwd, tt.path)
			if got != tt.want {
				t.Errorf("CanonicalFileMention(%q, %q) = %q, want %q",
					tt.cwd, tt.path, got, tt.want)
			}
		})
	}
}

func TestRegistryGetSet(t *testing.T) {
	const fakeCLI uxp.CLIName = "test-cli-registry"
	if got := GetMentionExtractor(fakeCLI); got != nil {
		t.Fatalf("expected nil before register, got %v", got)
	}
	called := false
	RegisterMentionExtractor(fakeCLI, func(ToolCall) []string {
		called = true
		return []string{"@file.x"}
	})
	t.Cleanup(func() {
		extractorMu.Lock()
		delete(extractors, fakeCLI)
		extractorMu.Unlock()
	})
	fn := GetMentionExtractor(fakeCLI)
	if fn == nil {
		t.Fatal("expected non-nil extractor after register")
	}
	got := fn(ToolCall{})
	if !called || !reflect.DeepEqual(got, []string{"@file.x"}) {
		t.Errorf("extractor not invoked correctly: called=%v got=%v",
			called, got)
	}
}

func TestExtractMentionsDedup(t *testing.T) {
	const fakeCLI uxp.CLIName = "test-cli-extract"
	RegisterMentionExtractor(fakeCLI, func(tc ToolCall) []string {
		switch tc.Name {
		case "Read":
			return []string{"@file.a", "@file.b"}
		case "Edit":
			return []string{"@file.b", "@file.c"}
		case "Empty":
			return nil
		}
		return nil
	})
	t.Cleanup(func() {
		extractorMu.Lock()
		delete(extractors, fakeCLI)
		extractorMu.Unlock()
	})

	calls := []ToolCall{
		{Name: "Read"}, {Name: "Edit"}, {Name: "Empty"}, {Name: "Read"},
	}
	got := ExtractMentions(fakeCLI, calls)
	want := []string{"@file.a", "@file.b", "@file.c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractMentions = %v, want %v", got, want)
	}

	if got := ExtractMentions("unknown-cli", calls); got != nil {
		t.Errorf("expected nil for unknown CLI, got %v", got)
	}
}

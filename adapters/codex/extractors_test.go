package codex

import (
	"encoding/json"
	"reflect"
	"testing"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

func TestCodexShellExtract(t *testing.T) {
	tests := []struct {
		name string
		tc   session.ToolCall
		want []string
	}{
		{
			"rel path arg",
			session.ToolCall{Name: "shell",
				Input: `{"command":["cat","cmd/main.go"],"cwd":"/proj"}`},
			[]string{"@file.cmd-main-go"},
		},
		{
			"abs in cwd",
			session.ToolCall{Name: "shell",
				Input: `{"command":["cat","/proj/a.go"],"cwd":"/proj"}`},
			[]string{"@file.a-go"},
		},
		{
			"abs outside cwd skipped",
			session.ToolCall{Name: "shell",
				Input: `{"command":["cat","/other/x.go"],"cwd":"/proj"}`},
			nil,
		},
		{
			"multiple paths",
			session.ToolCall{Name: "shell",
				Input: `{"command":["cp","a.go","b.go"],"cwd":"/proj"}`},
			[]string{"@file.a-go", "@file.b-go"},
		},
		{
			"flags skipped",
			session.ToolCall{Name: "shell",
				Input: `{"command":["ls","-la","x.go"],"cwd":"/proj"}`},
			[]string{"@file.x-go"},
		},
		{
			"bare command",
			session.ToolCall{Name: "shell",
				Input: `{"command":["pwd"],"cwd":"/proj"}`},
			nil,
		},
		{
			"malformed JSON",
			session.ToolCall{Name: "shell", Input: "{not"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := codexExtract(tt.tc)
			if !reflect.DeepEqual(got, tt.want) && (len(got) != 0 || len(tt.want) != 0) {
				t.Errorf("codexExtract = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodexApplyPatchExtract(t *testing.T) {
	patch := "*** Begin Patch\n" +
		"*** Update File: session/extractors.go\n" +
		"@@\n some context\n" +
		"*** Add File: adapters/codex/new.go\n" +
		"+content\n" +
		"*** Delete File: old/dead.go\n" +
		"*** End Patch\n"

	tests := []struct {
		name string
		tc   session.ToolCall
		want []string
	}{
		{
			"raw patch",
			session.ToolCall{Name: "apply_patch", Input: patch},
			[]string{
				"@file.session-extractors-go",
				"@file.adapters-codex-new-go",
				"@file.old-dead-go",
			},
		},
		{
			"wrapped {input:...}",
			session.ToolCall{Name: "apply_patch",
				Input: mustJSON(map[string]string{"input": patch})},
			[]string{
				"@file.session-extractors-go",
				"@file.adapters-codex-new-go",
				"@file.old-dead-go",
			},
		},
		{
			"empty patch",
			session.ToolCall{Name: "apply_patch", Input: ""},
			nil,
		},
		{
			"no headers",
			session.ToolCall{Name: "apply_patch",
				Input: "*** Begin Patch\n nothing here\n*** End Patch"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := codexExtract(tt.tc)
			if !reflect.DeepEqual(got, tt.want) && (len(got) != 0 || len(tt.want) != 0) {
				t.Errorf("codexExtract = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodexExtractRegistered(t *testing.T) {
	if session.GetMentionExtractor(uxp.CLICodex) == nil {
		t.Fatal("expected codex extractor registered")
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

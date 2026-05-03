package claude

import (
	"reflect"
	"testing"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

func TestClaudeExtract(t *testing.T) {
	tests := []struct {
		name string
		tc   session.ToolCall
		want []string
	}{
		{
			"Read file_path",
			session.ToolCall{Name: "Read",
				Input: `{"file_path":"session/envelope.go"}`},
			[]string{"@file.session-envelope-go"},
		},
		{
			"Edit file_path",
			session.ToolCall{Name: "Edit",
				Input: `{"file_path":"a/b.go","old_string":"x","new_string":"y"}`},
			[]string{"@file.a-b-go"},
		},
		{
			"Write file_path",
			session.ToolCall{Name: "Write",
				Input: `{"file_path":"out.txt","content":"hi"}`},
			[]string{"@file.out-txt"},
		},
		{
			"MultiEdit",
			session.ToolCall{Name: "MultiEdit",
				Input: `{"file_path":"x.go"}`},
			[]string{"@file.x-go"},
		},
		{
			"NotebookEdit notebook_path",
			session.ToolCall{Name: "NotebookEdit",
				Input: `{"notebook_path":"nb.ipynb"}`},
			[]string{"@file.nb-ipynb"},
		},
		{
			"Glob with path",
			session.ToolCall{Name: "Glob",
				Input: `{"pattern":"src/**/*.go"}`},
			[]string{"@file.src-**-*-go"},
		},
		{
			"Glob bare name skipped",
			session.ToolCall{Name: "Glob",
				Input: `{"pattern":"*.go"}`},
			nil,
		},
		{
			"Grep path field",
			session.ToolCall{Name: "Grep",
				Input: `{"pattern":"foo","path":"src/x.go"}`},
			[]string{"@file.src-x-go"},
		},
		{
			"Grep no path",
			session.ToolCall{Name: "Grep",
				Input: `{"pattern":"foo"}`},
			nil,
		},
		{
			"Unknown tool",
			session.ToolCall{Name: "Bash",
				Input: `{"command":"ls"}`},
			nil,
		},
		{
			"Empty input",
			session.ToolCall{Name: "Read", Input: ""},
			nil,
		},
		{
			"Malformed JSON",
			session.ToolCall{Name: "Read", Input: "{not-json"},
			nil,
		},
		{
			"Missing file_path",
			session.ToolCall{Name: "Read",
				Input: `{"other":"x"}`},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := claudeExtract(tt.tc)
			if !reflect.DeepEqual(got, tt.want) && !(len(got) == 0 && len(tt.want) == 0) {
				t.Errorf("claudeExtract = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaudeExtractRegisteredAndDedup(t *testing.T) {
	fn := session.GetMentionExtractor(uxp.CLIClaude)
	if fn == nil {
		t.Fatal("expected claude extractor registered")
	}
	calls := []session.ToolCall{
		{Name: "Read", Input: `{"file_path":"a.go"}`},
		{Name: "Edit", Input: `{"file_path":"a.go"}`},
		{Name: "Write", Input: `{"file_path":"b.go"}`},
		{Name: "Bash", Input: `{"command":"x"}`}, // skipped
	}
	got := session.ExtractMentions(uxp.CLIClaude, calls)
	want := []string{"@file.a-go", "@file.b-go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractMentions = %v, want %v", got, want)
	}
}

package gemini

import (
	"reflect"
	"testing"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

func TestGeminiExtract(t *testing.T) {
	tests := []struct {
		name string
		tc   session.ToolCall
		want []string
	}{
		{
			"file_path",
			session.ToolCall{Name: "read_file",
				Input: `{"file_path":"src/x.go"}`},
			[]string{"@file.src-x-go"},
		},
		{
			"path",
			session.ToolCall{Name: "list_directory",
				Input: `{"path":"docs"}`},
			[]string{"@file.docs"},
		},
		{
			"absolute_path with no cwd skipped",
			session.ToolCall{Name: "read_file",
				Input: `{"absolute_path":"/abs/p.go"}`},
			nil,
		},
		{
			"file_path + path dedup",
			session.ToolCall{Name: "x",
				Input: `{"file_path":"a.go","path":"a.go"}`},
			[]string{"@file.a-go"},
		},
		{
			"empty input",
			session.ToolCall{Name: "read_file"},
			nil,
		},
		{
			"malformed",
			session.ToolCall{Name: "read_file", Input: "{x"},
			nil,
		},
		{
			"no recognized key",
			session.ToolCall{Name: "x",
				Input: `{"other":"a.go"}`},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := geminiExtract(tt.tc)
			if !reflect.DeepEqual(got, tt.want) && (len(got) != 0 || len(tt.want) != 0) {
				t.Errorf("geminiExtract = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGeminiExtractRegistered(t *testing.T) {
	if session.GetMentionExtractor(uxp.CLIGemini) == nil {
		t.Fatal("expected gemini extractor registered")
	}
}

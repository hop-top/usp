package opencode

import (
	"reflect"
	"testing"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

func TestOpencodeExtract(t *testing.T) {
	tests := []struct {
		name string
		tc   session.ToolCall
		want []string
	}{
		{
			"file_path",
			session.ToolCall{Name: "edit",
				Input: `{"file_path":"src/x.go"}`},
			[]string{"@file.src-x-go"},
		},
		{
			"path",
			session.ToolCall{Name: "ls",
				Input: `{"path":"docs"}`},
			[]string{"@file.docs"},
		},
		{
			"file_path + path dedup",
			session.ToolCall{Name: "x",
				Input: `{"file_path":"a.go","path":"a.go"}`},
			[]string{"@file.a-go"},
		},
		{
			"empty",
			session.ToolCall{Name: "edit"},
			nil,
		},
		{
			"malformed",
			session.ToolCall{Name: "edit", Input: "{"},
			nil,
		},
		{
			"unrecognised key",
			session.ToolCall{Name: "edit",
				Input: `{"target":"a.go"}`},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := opencodeExtract(tt.tc)
			if !reflect.DeepEqual(got, tt.want) && !(len(got) == 0 && len(tt.want) == 0) {
				t.Errorf("opencodeExtract = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpencodeExtractRegistered(t *testing.T) {
	if session.GetMentionExtractor(uxp.CLIOpenCode) == nil {
		t.Fatal("expected opencode extractor registered")
	}
}

package gemini

import (
	"encoding/json"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

// init registers the Gemini mention extractor.
//
// Gemini tools (read_file, write_file, list_directory, etc.) carry
// path info in any of: file_path, path, absolute_path. Extractor
// pulls whichever is present. Inputs are JSON; on parse failure the
// call is skipped silently.
func init() {
	session.RegisterMentionExtractor(uxp.CLIGemini, geminiExtract)
}

func geminiExtract(tc session.ToolCall) []string {
	if tc.Input == "" {
		return nil
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(tc.Input), &in); err != nil {
		return nil
	}
	keys := []string{"file_path", "path", "absolute_path"}
	var out []string
	seen := make(map[string]struct{})
	for _, k := range keys {
		v, ok := in[k].(string)
		if !ok || v == "" {
			continue
		}
		m := session.CanonicalFileMention("", v)
		if m == "" {
			continue
		}
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

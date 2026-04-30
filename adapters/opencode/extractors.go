package opencode

import (
	"encoding/json"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// init registers the OpenCode mention extractor.
//
// OpenCode parts surface tool args in JSON. Extractor pulls
// file_path / path. Unknown shapes are skipped silently.
func init() {
	session.RegisterMentionExtractor(uxp.CLIOpenCode, opencodeExtract)
}

func opencodeExtract(tc session.ToolCall) []string {
	if tc.Input == "" {
		return nil
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(tc.Input), &in); err != nil {
		return nil
	}
	keys := []string{"file_path", "path"}
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

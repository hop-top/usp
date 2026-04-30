package claude

import (
	"encoding/json"
	"strings"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// init registers the Claude mention extractor.
//
// Tool coverage: Read, Edit, Write, MultiEdit, NotebookEdit, Glob,
// Grep. Pulls file_path / notebook_path. Glob uses pattern only when
// it looks like a path (contains /). Grep pattern is a regex — never
// a file mention. Tool input is parsed JSON; on parse failure the
// call is skipped silently.
//
// Path handling: extractor passes the raw value through
// session.CanonicalFileMention with empty projectCwd — paths are
// treated as already-relative. Caller-side projectCwd resolution is
// future work.
func init() {
	session.RegisterMentionExtractor(uxp.CLIClaude, claudeExtract)
}

func claudeExtract(tc session.ToolCall) []string {
	if tc.Input == "" {
		return nil
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(tc.Input), &in); err != nil {
		return nil
	}
	var paths []string
	switch tc.Name {
	case "Read", "Edit", "Write", "MultiEdit":
		if p, ok := in["file_path"].(string); ok {
			paths = append(paths, p)
		}
	case "NotebookEdit":
		if p, ok := in["notebook_path"].(string); ok {
			paths = append(paths, p)
		}
		if p, ok := in["file_path"].(string); ok {
			paths = append(paths, p)
		}
	case "Glob":
		if p, ok := in["pattern"].(string); ok && looksLikePath(p) {
			paths = append(paths, p)
		}
	case "Grep":
		// pattern is a regex; only path field is "path".
		if p, ok := in["path"].(string); ok {
			paths = append(paths, p)
		}
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if m := session.CanonicalFileMention("", p); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// looksLikePath reports whether a glob pattern names a path (vs. a
// bare basename like "*.go"). Heuristic: contains "/".
func looksLikePath(s string) bool {
	return strings.Contains(s, "/")
}

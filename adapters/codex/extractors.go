package codex

import (
	"encoding/json"
	"strings"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// init registers the Codex mention extractor.
//
// Tool patterns:
//   - shell: parse cwd + first arg from JSON Input; if arg looks
//     like a path, mention it (joined to cwd when relative).
//   - apply_patch: scan Input for "*** Update File: <p>",
//     "*** Add File: <p>", "*** Delete File: <p>" headers.
//
// Unparseable inputs are skipped silently.
func init() {
	session.RegisterMentionExtractor(uxp.CLICodex, codexExtract)
}

func codexExtract(tc session.ToolCall) []string {
	if tc.Input == "" {
		return nil
	}
	switch tc.Name {
	case "shell":
		return codexShell(tc.Input)
	case "apply_patch":
		return codexApplyPatch(tc.Input)
	}
	return nil
}

func codexShell(input string) []string {
	var args struct {
		Cwd     string   `json:"cwd"`
		Command []string `json:"command"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return nil
	}
	var out []string
	for _, a := range args.Command {
		if !looksLikePath(a) {
			continue
		}
		if m := session.CanonicalFileMention(args.Cwd, a); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// codexApplyPatch parses the v4 patch envelope used by Codex's
// apply_patch tool. Headers of interest:
//
//	*** Update File: <path>
//	*** Add File: <path>
//	*** Delete File: <path>
func codexApplyPatch(input string) []string {
	// Input may be wrapped JSON like {"input":"...patch..."} or raw.
	patch := input
	var wrap struct {
		Input string `json:"input"`
		Patch string `json:"patch"`
	}
	if err := json.Unmarshal([]byte(input), &wrap); err == nil {
		switch {
		case wrap.Input != "":
			patch = wrap.Input
		case wrap.Patch != "":
			patch = wrap.Patch
		}
	}

	var out []string
	for _, line := range strings.Split(patch, "\n") {
		p := parsePatchHeader(line)
		if p == "" {
			continue
		}
		if m := session.CanonicalFileMention("", p); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func parsePatchHeader(line string) string {
	prefixes := []string{
		"*** Update File: ", "*** Add File: ", "*** Delete File: ",
	}
	for _, pre := range prefixes {
		if strings.HasPrefix(line, pre) {
			return strings.TrimSpace(strings.TrimPrefix(line, pre))
		}
	}
	return ""
}

// looksLikePath reports whether a shell arg likely names a file
// (contains "/" or "."). Skips bare flags / commands.
func looksLikePath(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	return strings.Contains(s, "/") || strings.Contains(s, ".")
}

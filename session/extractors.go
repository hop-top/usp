package session

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"hop.top/kit/go/core/uxp"
)

// MentionExtractor returns canonical @file.<slug> mentions for a tool
// call. Empty result = no extractable file mentions.
type MentionExtractor func(ToolCall) []string

var (
	extractorMu sync.RWMutex
	extractors  = make(map[uxp.CLIName]MentionExtractor)
)

// RegisterMentionExtractor binds an extractor to a CLI name. Last
// registration wins. Safe to call from init().
func RegisterMentionExtractor(cli uxp.CLIName, fn MentionExtractor) {
	extractorMu.Lock()
	defer extractorMu.Unlock()
	extractors[cli] = fn
}

// GetMentionExtractor returns the extractor for a CLI, or nil if
// none registered.
func GetMentionExtractor(cli uxp.CLIName) MentionExtractor {
	extractorMu.RLock()
	defer extractorMu.RUnlock()
	return extractors[cli]
}

// ExtractMentions runs the registered extractor over every tool call
// and returns deduped mentions in first-seen order. Returns nil when
// no extractor is registered for the CLI.
func ExtractMentions(cli uxp.CLIName, calls []ToolCall) []string {
	fn := GetMentionExtractor(cli)
	if fn == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	for _, c := range calls {
		for _, m := range fn(c) {
			if m == "" {
				continue
			}
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out
}

// ansiRE matches CSI escape sequences (color/cursor codes).
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes CSI escape sequences from s.
func StripANSI(s string) string {
	if s == "" {
		return ""
	}
	return ansiRE.ReplaceAllString(s, "")
}

// CanonicalFileMention returns "@file.<slug>" for a path. Slug =
// lowercased relpath with "/" and "." → "-". When projectCwd is
// empty, absOrRelPath is treated as already-relative. Returns ""
// for empty input or absolute paths outside projectCwd.
func CanonicalFileMention(projectCwd, absOrRelPath string) string {
	if absOrRelPath == "" {
		return ""
	}
	rel := absOrRelPath
	if filepath.IsAbs(absOrRelPath) {
		if projectCwd == "" {
			return ""
		}
		r, err := filepath.Rel(projectCwd, absOrRelPath)
		if err != nil || strings.HasPrefix(r, "..") {
			return ""
		}
		rel = r
	}
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.ToLower(rel)
	rel = strings.ReplaceAll(rel, "/", "-")
	rel = strings.ReplaceAll(rel, ".", "-")
	if rel == "" {
		return ""
	}
	return "@file." + rel
}

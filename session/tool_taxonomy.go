package session

import (
	"strings"
	"sync"

	"hop.top/kit/go/core/uxp"
	"hop.top/kit/go/core/uxp/invoke"
	kitclaude "hop.top/kit/go/core/uxp/invoke/adapters/claude"
	kitcodex "hop.top/kit/go/core/uxp/invoke/adapters/codex"
	kitcopilot "hop.top/kit/go/core/uxp/invoke/adapters/copilot"
	kitcrush "hop.top/kit/go/core/uxp/invoke/adapters/crush"
	kitcursor "hop.top/kit/go/core/uxp/invoke/adapters/cursoragent"
	kitgemini "hop.top/kit/go/core/uxp/invoke/adapters/gemini"
	kitgoose "hop.top/kit/go/core/uxp/invoke/adapters/goose"
	kitkimi "hop.top/kit/go/core/uxp/invoke/adapters/kimi"
	kitopencode "hop.top/kit/go/core/uxp/invoke/adapters/opencode"
	kitqwen "hop.top/kit/go/core/uxp/invoke/adapters/qwen"
	kitvibe "hop.top/kit/go/core/uxp/invoke/adapters/vibe"
)

// ToolCapabilityFor returns the cross-CLI ToolCapability matching
// the given native tool name on the given CLI, plus a found flag.
// Native lookup is case-sensitive against the adapter's documented
// NativeNames slice; that's deliberate because adapters tag tools
// using the literal native names that show up in transcripts.
//
// The taxonomy lives in kit's invoke package; this is a thin
// shortcut for usp's transcript renderer.
func ToolCapabilityFor(cli uxp.CLIName, nativeName string) (invoke.ToolCapability, bool) {
	a, ok := taxonomyAdapters()[cli]
	if !ok {
		return invoke.ToolCapability{}, false
	}
	for _, c := range a.ToolCapabilities() {
		for _, n := range c.NativeNames {
			if n == nativeName {
				return c, true
			}
		}
	}
	return invoke.ToolCapability{}, false
}

// UniversalToolLabel returns the rendering label for a tool call:
//   - "shell.exec (Bash)"            when the universal name and a
//     concrete native name are known,
//   - "[native:Bash]"                 when the CLI is unknown or no
//     mapping exists.
//
// The label is stable: same inputs → same string, suitable for
// transcript rendering and snapshot tests.
func UniversalToolLabel(cli uxp.CLIName, nativeName string) string {
	if c, ok := ToolCapabilityFor(cli, nativeName); ok && c.Universal != "" {
		return c.Universal + " (" + nativeName + ")"
	}
	return "[native:" + nativeName + "]"
}

// UniversalToolName returns the universal name for a native tool, or
// the empty string if no mapping is known. Useful when rendering
// structured (JSON) output that wants Universal as its own field
// rather than a composed label.
func UniversalToolName(cli uxp.CLIName, nativeName string) string {
	if c, ok := ToolCapabilityFor(cli, nativeName); ok {
		return c.Universal
	}
	return ""
}

// taxonomyAdapters returns the singleton CLI→adapter map used by
// transcript rendering. Initialized once, read-only thereafter.
//
// The map is keyed by uxp.CLIName so callers pass canonical names
// (claude, gemini, etc.). Detection-only registry entries (amp,
// antigravity, tabnine, windsurf) have no invocation adapter and
// therefore no taxonomy here; ToolCapabilityFor returns ok=false
// for them.
var (
	taxonomyOnce  sync.Once
	taxonomyCache map[uxp.CLIName]invoke.InvocationAdapter
)

func taxonomyAdapters() map[uxp.CLIName]invoke.InvocationAdapter {
	taxonomyOnce.Do(func() {
		all := []invoke.InvocationAdapter{
			kitclaude.New(), kitgemini.New(), kitcodex.New(),
			kitopencode.New(), kitcopilot.New(), kitcursor.New(),
			kitqwen.New(), kitkimi.New(), kitvibe.New(),
			kitgoose.New(), kitcrush.New(),
		}
		taxonomyCache = make(map[uxp.CLIName]invoke.InvocationAdapter, len(all))
		for _, a := range all {
			taxonomyCache[a.CLI()] = a
		}
	})
	return taxonomyCache
}

// CategorizeUniversalTool returns a coarse activity category for a
// universal tool name: "research" / "edit" / "exec" / "todo" /
// "task" / "" (unknown). Useful for transcript summaries that group
// tool calls by intent rather than name.
//
// This replaces the older string-matching categorizer in
// internal/api/session_items.go for callers willing to use the
// taxonomy.
func CategorizeUniversalTool(universal string) string {
	switch universal {
	case "file.read", "file.search", "web.search", "web.fetch":
		return "research"
	case "file.write", "file.edit":
		return "edit"
	case "shell.exec":
		return "exec"
	case "todo.write":
		return "todo"
	case "task.spawn", "plan.update":
		return "task"
	}
	if strings.HasPrefix(universal, "browser.") {
		return "browser"
	}
	return ""
}

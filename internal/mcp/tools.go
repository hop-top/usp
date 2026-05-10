package mcp

func tools() []map[string]any {
	return []map[string]any{
		{
			"name":        "usp_session_list",
			"description": "List normalized USP sessions across supported local CLIs.",
			"inputSchema": objectSchema(map[string]any{
				"project": stringProp("Restrict to a project cwd."),
				"cli":     stringProp("Restrict to one CLI: claude, codex, gemini, or opencode."),
				"since":   stringProp("Only include sessions since a date or duration, such as 2026-05-01 or 7d."),
				"limit":   integerProp("Maximum sessions to return."),
			}, nil),
		},
		{
			"name":        "usp_session_search",
			"description": "Search session turn text across supported local CLIs.",
			"inputSchema": objectSchema(map[string]any{
				"query":   stringProp("Text to search for."),
				"project": stringProp("Restrict to a project cwd."),
				"cli":     stringProp("Restrict to one CLI: claude, codex, gemini, or opencode."),
				"since":   stringProp("Only search sessions since a date or duration."),
				"limit":   integerProp("Maximum sessions to return."),
			}, []string{"query"}),
		},
		{
			"name":        "usp_session_show",
			"description": "Show one normalized USP session with turns and optional skills.",
			"inputSchema": objectSchema(map[string]any{
				"id":             stringProp("Canonical USP session id, native session id, or unambiguous prefix."),
				"project":        stringProp("Restrict lookup to a project cwd."),
				"cli":            stringProp("Restrict lookup to one CLI."),
				"since":          stringProp("Only match sessions since a date or duration."),
				"include_skills": boolProp("Include extracted skill invocation events."),
			}, []string{"id"}),
		},
		{
			"name":        "usp_session_skills",
			"description": "List skill invocation events across matching sessions.",
			"inputSchema": objectSchema(map[string]any{
				"session": stringProp("Restrict to one session id or prefix."),
				"project": stringProp("Restrict to a project cwd."),
				"cli":     stringProp("Restrict to one CLI."),
				"name":    stringProp("Restrict to a skill name substring."),
				"since":   stringProp("Only include events since a date or duration."),
				"until":   stringProp("Only include events until a date or duration."),
			}, nil),
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func stringProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerProp(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func boolProp(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

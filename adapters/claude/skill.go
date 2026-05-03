package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

// commandNameRE captures slash-command invocations injected into
// user content, e.g. <command-name>/excalidraw</command-name>.
var commandNameRE = regexp.MustCompile(`<command-name>/?([^<\s]+)</command-name>`)

// commandArgsRE captures the optional <command-args>...</command-args>
// block that travels with a slash command. Multi-line content is
// matched via the (?s) flag.
var commandArgsRE = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)

const triggerQueryMaxLen = 240

// ExtractSkills returns every skill invocation in the session.
// Two patterns are surfaced:
//
//  1. Slash-command user turns: <command-name>/foo</command-name>.
//     The owning user turn is the trigger; outcome is "invoked"
//     (the command always runs once parsed) and the trigger query
//     is the <command-args> payload (truncated).
//
//  2. Skill tool calls in assistant turns: tool_use blocks with
//     name="Skill". The user turn immediately preceding the
//     assistant turn is the trigger; outcome is "errored" if the
//     matching tool_result has is_error=true, "invoked" otherwise.
func (a *Adapter) ExtractSkills(sessionID string) ([]session.SkillEvent, error) {
	path, err := a.findSessionFile(sessionID)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var (
		events           []session.SkillEvent
		lastUserTurnUUID string
		lastUserContent  string
	)

	// toolUseTurn holds a pending Skill tool_use awaiting its
	// matching tool_result for outcome resolution.
	type toolUseTurn struct {
		idx int // index in events
		id  string
	}
	pendingByID := map[string]toolUseTurn{}

	for scanner.Scan() {
		var ev jsonlEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "user":
			lastUserTurnUUID = ev.UUID
			content := userTurnText(ev.Message)
			lastUserContent = content

			// Slash-command turns: emit immediately.
			if name, args, ok := parseSlashCommand(content); ok {
				ts := parseTimestamp(ev.Timestamp)
				events = append(events, session.SkillEvent{
					SessionID:     sessionID,
					CLI:           uxp.CLIClaude,
					Timestamp:     ts,
					Name:          name,
					TriggerTurnID: ev.UUID,
					TriggerQuery:  truncate(args, triggerQueryMaxLen),
					Outcome:       session.SkillInvoked,
				})
			}

			// Resolve any pending Skill tool_results carried in
			// this user turn (Claude posts results as user turns).
			if results := extractSkillToolResults(ev.Message); len(results) > 0 {
				for id, errored := range results {
					p, ok := pendingByID[id]
					if !ok {
						continue
					}
					if errored {
						events[p.idx].Outcome = session.SkillErrored
					}
					delete(pendingByID, id)
				}
			}

		case "assistant":
			calls := assistantSkillCalls(ev.Message)
			if len(calls) == 0 {
				continue
			}
			ts := parseTimestamp(ev.Timestamp)
			for _, c := range calls {
				ev := session.SkillEvent{
					SessionID:     sessionID,
					CLI:           uxp.CLIClaude,
					Timestamp:     ts,
					Name:          c.name,
					TriggerTurnID: lastUserTurnUUID,
					TriggerQuery:  truncate(lastUserContent, triggerQueryMaxLen),
					Outcome:       session.SkillInvoked,
				}
				events = append(events, ev)
				if c.id != "" {
					pendingByID[c.id] = toolUseTurn{idx: len(events) - 1, id: c.id}
				}
			}
		}
	}
	return events, nil
}

// parseSlashCommand returns (name, args, ok) for a user-turn body
// that injected a slash command. The name has the leading slash
// stripped; args is the <command-args> payload (may be empty).
func parseSlashCommand(content string) (string, string, bool) {
	m := commandNameRE.FindStringSubmatch(content)
	if len(m) < 2 || m[1] == "" {
		return "", "", false
	}
	args := ""
	if am := commandArgsRE.FindStringSubmatch(content); len(am) >= 2 {
		args = strings.TrimSpace(am[1])
	}
	return m[1], args, true
}

// userTurnText extracts plain text from a user message payload,
// preserving the embedded <command-*> markup so [parseSlashCommand]
// can scan it.
func userTurnText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var msg messagePayload
	if err := json.Unmarshal(raw, &msg); err != nil {
		return ""
	}
	// Plain string content.
	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil {
		return s
	}
	// Block array — concat any "text" blocks.
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

type skillCall struct {
	id   string
	name string
}

// assistantSkillCalls returns Skill tool_use blocks from an
// assistant message payload.
func assistantSkillCalls(raw json.RawMessage) []skillCall {
	if len(raw) == 0 {
		return nil
	}
	var msg messagePayload
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	var blocks []struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil
	}
	var calls []skillCall
	for _, b := range blocks {
		if b.Type != "tool_use" || b.Name != "Skill" {
			continue
		}
		var input struct {
			Skill string `json:"skill"`
		}
		_ = json.Unmarshal(b.Input, &input)
		name := input.Skill
		if name == "" {
			name = "Skill"
		}
		calls = append(calls, skillCall{id: b.ID, name: name})
	}
	return calls
}

// extractSkillToolResults returns map[tool_use_id] -> isError for
// every tool_result block in a user turn. Used to upgrade pending
// "invoked" events to "errored" when the matching result reports
// is_error=true.
func extractSkillToolResults(raw json.RawMessage) map[string]bool {
	if len(raw) == 0 {
		return nil
	}
	var msg messagePayload
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}
	var blocks []struct {
		Type      string `json:"type"`
		ToolUseID string `json:"tool_use_id"`
		IsError   bool   `json:"is_error"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil
	}
	out := make(map[string]bool)
	for _, b := range blocks {
		if b.Type != "tool_result" || b.ToolUseID == "" {
			continue
		}
		out[b.ToolUseID] = b.IsError
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

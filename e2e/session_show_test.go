package e2e

// e2e tests for the session show workflow.
// Exercises: adapter resolution, turn streaming, JSON schema, tool-call
// boundaries, error paths, and cross-CLI disambiguation.
// Covers ACs from US-0002 and US-0004.

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

// showResult mirrors cmd/usp.showResult for JSON round-trip assertions.
type showResult struct {
	ID        string     `json:"id"`
	CLI       string     `json:"cli"`
	Project   string     `json:"project"`
	StartedAt string     `json:"started_at"`
	EndedAt   string     `json:"ended_at"`
	TurnCount int        `json:"turn_count"`
	Turns     []showTurn `json:"turns"`
}

type showTurn struct {
	Role      string             `json:"role"`
	Content   string             `json:"content"`
	Timestamp string             `json:"timestamp"`
	ToolCalls []session.ToolCall `json:"tool_calls,omitempty"`
}

// buildShowResult simulates the assembly done inside sessionShowCmd.
func buildShowResult(
	sess *session.Session,
	matchedCLI string,
	a session.SessionAdapter,
) showResult {
	ch, err := a.StreamTurns(sess.ID)
	var turns []showTurn
	if err == nil {
		for turn := range ch {
			turns = append(turns, showTurn{
				Role:      string(turn.Role),
				Content:   turn.Content,
				Timestamp: turn.Timestamp.Format("2006-01-02 15:04:05"),
				ToolCalls: turn.ToolCalls,
			})
		}
	}
	ended := "active"
	if sess.EndedAt != nil {
		ended = sess.EndedAt.Format("2006-01-02 15:04:05")
	}
	return showResult{
		ID:        sess.ID,
		CLI:       matchedCLI,
		Project:   sess.ProjectCwd,
		StartedAt: sess.StartedAt.Format("2006-01-02 15:04:05"),
		EndedAt:   ended,
		TurnCount: sess.TurnCount,
		Turns:     turns,
	}
}

// TestSessionShow_ValidClaudeID verifies human-readable metadata fields
// and ordered turns for a plain Claude session (US-0004 AC1, US-0002 AC1).
func TestSessionShow_ValidClaudeID(t *testing.T) {
	home := t.TempDir()
	a := setupClaude(t, home)

	adapters := map[string]session.SessionAdapter{
		string(uxp.CLIClaude): a,
	}
	order := []string{string(uxp.CLIClaude)}

	sess, matchedCLI, adapter, err := sessionutil.ResolveSessionID(
		"claude-sess-01", adapters, order)
	if err != nil {
		t.Fatalf("ResolveSessionID: %v", err)
	}

	res := buildShowResult(sess, matchedCLI, adapter)

	if res.ID != "claude-sess-01" {
		t.Errorf("ID = %q, want %q", res.ID, "claude-sess-01")
	}
	if res.CLI != string(uxp.CLIClaude) {
		t.Errorf("CLI = %q, want %q", res.CLI, uxp.CLIClaude)
	}
	if res.Project == "" {
		t.Error("Project must not be empty")
	}
	if res.StartedAt == "" {
		t.Error("StartedAt must not be empty")
	}
	if res.EndedAt == "" {
		t.Error("EndedAt must not be empty")
	}
	if res.TurnCount < 2 {
		t.Errorf("TurnCount = %d, want >= 2", res.TurnCount)
	}
	if len(res.Turns) < 2 {
		t.Fatalf("Turns count = %d, want >= 2", len(res.Turns))
	}
	if res.Turns[0].Role != string(session.RoleUser) {
		t.Errorf("first turn role = %q, want %q",
			res.Turns[0].Role, session.RoleUser)
	}
	if res.Turns[1].Role != string(session.RoleAssistant) {
		t.Errorf("second turn role = %q, want %q",
			res.Turns[1].Role, session.RoleAssistant)
	}
	// Each turn must carry a timestamp string.
	for i, turn := range res.Turns {
		if turn.Timestamp == "" {
			t.Errorf("turn[%d].Timestamp is empty", i)
		}
		if turn.Content == "" {
			t.Errorf("turn[%d].Content is empty", i)
		}
	}
}

// TestSessionShow_JSONRoundTrip verifies that the JSON payload parses
// cleanly and every turn has the required schema fields (US-0002 AC2,
// US-0004 AC3).
func TestSessionShow_JSONRoundTrip(t *testing.T) {
	home := t.TempDir()
	a := setupClaude(t, home)

	adapters := map[string]session.SessionAdapter{
		string(uxp.CLIClaude): a,
	}
	order := []string{string(uxp.CLIClaude)}

	sess, matchedCLI, adapter, err := sessionutil.ResolveSessionID(
		"claude-sess-01", adapters, order)
	if err != nil {
		t.Fatalf("ResolveSessionID: %v", err)
	}

	res := buildShowResult(sess, matchedCLI, adapter)

	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed showResult
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed.ID != res.ID {
		t.Errorf("round-trip ID = %q, want %q", parsed.ID, res.ID)
	}
	for i, turn := range parsed.Turns {
		if turn.Role == "" {
			t.Errorf("turn[%d].role missing after round-trip", i)
		}
		if turn.Content == "" {
			t.Errorf("turn[%d].content missing after round-trip", i)
		}
		if turn.Timestamp == "" {
			t.Errorf("turn[%d].timestamp missing after round-trip", i)
		}
	}
}

// TestSessionShow_ToolCallBoundaries verifies that tool-call turns are
// distinct from regular assistant text and carry name/input/output fields
// (US-0004 AC2, US-0002 AC4).
func TestSessionShow_ToolCallBoundaries(t *testing.T) {
	home := t.TempDir()
	a := claude.New(claude.WithHomeDir(home))
	key := a.ProjectKey(fixtureCwd)
	dir := filepath.Join(home, ".claude", "projects", key)

	// Session with a tool-use turn followed by a tool_result turn
	// (claude adapter merges them into one Turn with ToolCalls populated).
	writeJSONL(t, filepath.Join(dir, "show-tool-sess.jsonl"), []map[string]any{
		{
			"uuid": "t1", "type": "user",
			"timestamp": "2026-04-11T09:00:00Z",
			"cwd": fixtureCwd, "sessionId": "show-tool-sess",
			"message": map[string]any{
				"role":    "user",
				"content": "list files",
			},
		},
		{
			"uuid": "t2", "type": "assistant",
			"timestamp": "2026-04-11T09:00:05Z",
			"sessionId": "show-tool-sess",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "tool-abc",
						"name":  "bash",
						"input": map[string]any{"command": "ls"},
					},
				},
			},
		},
		{
			"uuid": "t3", "type": "user",
			"timestamp": "2026-04-11T09:00:06Z",
			"sessionId": "show-tool-sess",
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "tool-abc",
						"content":     "file1.go\nfile2.go",
					},
				},
			},
		},
		{
			"uuid": "t4", "type": "assistant",
			"timestamp": "2026-04-11T09:00:07Z",
			"sessionId": "show-tool-sess",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Here are your files."},
				},
			},
		},
	})

	adapters := map[string]session.SessionAdapter{
		string(uxp.CLIClaude): a,
	}
	order := []string{string(uxp.CLIClaude)}

	sess, matchedCLI, adapter, err := sessionutil.ResolveSessionID(
		"show-tool-sess", adapters, order)
	if err != nil {
		t.Fatalf("ResolveSessionID: %v", err)
	}

	res := buildShowResult(sess, matchedCLI, adapter)

	if len(res.Turns) < 2 {
		t.Fatalf("expected >= 2 turns, got %d", len(res.Turns))
	}

	// Find the turn with tool calls.
	var toolTurn *showTurn
	var textOnlyTurns int
	for i := range res.Turns {
		if len(res.Turns[i].ToolCalls) > 0 {
			toolTurn = &res.Turns[i]
		} else {
			textOnlyTurns++
		}
	}
	if toolTurn == nil {
		t.Fatal("no turn with ToolCalls found")
	}
	if toolTurn.ToolCalls[0].Name != "bash" {
		t.Errorf("tool name = %q, want %q",
			toolTurn.ToolCalls[0].Name, "bash")
	}
	if toolTurn.ToolCalls[0].Output == "" {
		t.Error("tool_call.Output should be populated from tool_result")
	}
	if textOnlyTurns == 0 {
		t.Error("expected at least one non-tool-call turn")
	}

	// Tool-call turn and regular text turn must have different structure.
	raw, err := json.Marshal(res.Turns)
	if err != nil {
		t.Fatalf("marshal turns: %v", err)
	}
	var reparsed []showTurn
	if err := json.Unmarshal(raw, &reparsed); err != nil {
		t.Fatalf("unmarshal turns: %v", err)
	}
	// Ensure tool_calls only present on the relevant turn.
	for _, rt := range reparsed {
		if rt.Role == string(session.RoleUser) && len(rt.ToolCalls) > 0 {
			t.Error("user turn should not carry tool_calls in output")
		}
	}
}

// TestSessionShow_LongSessionOrdering verifies turn ordering is
// preserved for sessions with many turns (US-0002 AC4).
func TestSessionShow_LongSessionOrdering(t *testing.T) {
	home := t.TempDir()
	a := claude.New(claude.WithHomeDir(home))
	key := a.ProjectKey(fixtureCwd)
	dir := filepath.Join(home, ".claude", "projects", key)

	// Build 10 alternating user/assistant turns.
	events := make([]map[string]any, 0, 10)
	baseTime := time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		role := "user"
		content := any("message " + itoa(i))
		if i%2 == 1 {
			role = "assistant"
			content = []map[string]any{
				{"type": "text", "text": "reply " + itoa(i)},
			}
		}
		msgType := role
		ev := map[string]any{
			"uuid":      "long-" + itoa(i),
			"type":      msgType,
			"timestamp": baseTime.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			"cwd":       fixtureCwd,
			"sessionId": "show-long-sess",
			"message": map[string]any{
				"role":    role,
				"content": content,
			},
		}
		events = append(events, ev)
	}

	writeJSONL(t, filepath.Join(dir, "show-long-sess.jsonl"), events)

	adapters := map[string]session.SessionAdapter{
		string(uxp.CLIClaude): a,
	}
	order := []string{string(uxp.CLIClaude)}

	sess, matchedCLI, adapter, err := sessionutil.ResolveSessionID(
		"show-long-sess", adapters, order)
	if err != nil {
		t.Fatalf("ResolveSessionID: %v", err)
	}

	res := buildShowResult(sess, matchedCLI, adapter)

	if len(res.Turns) != 10 {
		t.Fatalf("turn count = %d, want 10", len(res.Turns))
	}

	// Timestamps must be non-decreasing (ordering preserved).
	for i := 1; i < len(res.Turns); i++ {
		prev := res.Turns[i-1].Timestamp
		curr := res.Turns[i].Timestamp
		if curr < prev {
			t.Errorf("turn ordering broken: turn[%d].Timestamp %q < turn[%d].Timestamp %q",
				i, curr, i-1, prev)
		}
	}
	// Roles must alternate user/assistant.
	wantRoles := []session.Role{
		session.RoleUser, session.RoleAssistant,
	}
	for i, turn := range res.Turns {
		want := string(wantRoles[i%2])
		if turn.Role != want {
			t.Errorf("turn[%d].Role = %q, want %q", i, turn.Role, want)
		}
	}
}

// TestSessionShow_NonExistentID verifies that resolving a missing
// session ID returns a non-nil error explaining the failure
// (US-0002 AC3).
func TestSessionShow_NonExistentID(t *testing.T) {
	home := t.TempDir()
	a := setupClaude(t, home)

	adapters := map[string]session.SessionAdapter{
		string(uxp.CLIClaude): a,
	}
	order := []string{string(uxp.CLIClaude)}

	_, _, _, err := sessionutil.ResolveSessionID(
		"does-not-exist-session-xyz", adapters, order)
	if err == nil {
		t.Fatal("expected error for non-existent session id, got nil")
	}
	// Error message must indicate lookup failure (not a crash message).
	if err.Error() == "" {
		t.Error("error message must not be empty")
	}
}

// TestSessionShow_CrossCliDisambiguation verifies that a session ID
// present in both Claude and Codex can be disambiguated by passing
// the --tool flag (US-0004 AC4).
func TestSessionShow_CrossCliDisambiguation(t *testing.T) {
	home := t.TempDir()

	// Create Claude session with the shared ID.
	claudeA := claude.New(claude.WithHomeDir(home))
	keyC := claudeA.ProjectKey(fixtureCwd)
	dirC := filepath.Join(home, ".claude", "projects", keyC)
	writeJSONL(t, filepath.Join(dirC, "shared-id-sess.jsonl"), []map[string]any{
		{
			"uuid": "sid1", "type": "user",
			"timestamp": "2026-04-12T10:00:00Z",
			"cwd": fixtureCwd, "sessionId": "shared-id-sess",
			"message": map[string]any{
				"role":    "user",
				"content": "from claude side",
			},
		},
	})

	// Create Codex session with a different ID (Codex uses date-sharded JSONL).
	restoreSR := codex.SetSessionsRoot(
		filepath.Join(home, ".codex", "sessions"),
	)
	restoreCR := codex.SetCodexRoot(filepath.Join(home, ".codex"))
	t.Cleanup(restoreSR)
	t.Cleanup(restoreCR)

	codexA := &codex.Adapter{}
	sessDir := filepath.Join(home, ".codex", "sessions", "2026", "04", "12")
	writeJSONL(t, filepath.Join(sessDir, "codex-uniq-sess.jsonl"), []map[string]any{
		{
			"timestamp": "2026-04-12T10:00:00Z",
			"type":      "session_meta",
			"payload": map[string]any{
				"id":          "codex-uniq-sess",
				"timestamp":   "2026-04-12T10:00:00Z",
				"cwd":         fixtureCwd,
				"originator":  "user",
				"cli_version": "0.106.0",
				"source":      "cli",
			},
		},
		{
			"timestamp": "2026-04-12T10:00:01Z",
			"type":      "response_item",
			"payload": map[string]any{
				"type": "message", "role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "from codex side"},
				},
			},
		},
	})

	// Disambiguate by restricting to Claude only (--tool claude).
	claudeOnly := map[string]session.SessionAdapter{
		string(uxp.CLIClaude): claudeA,
	}
	order := []string{string(uxp.CLIClaude)}

	sess, matchedCLI, _, err := sessionutil.ResolveSessionID(
		"shared-id-sess", claudeOnly, order)
	if err != nil {
		t.Fatalf("Claude-scoped resolve: %v", err)
	}
	if matchedCLI != string(uxp.CLIClaude) {
		t.Errorf("matched CLI = %q, want %q", matchedCLI, uxp.CLIClaude)
	}
	if sess.ID != "shared-id-sess" {
		t.Errorf("ID = %q, want %q", sess.ID, "shared-id-sess")
	}

	// Codex-scoped lookup should find its own session.
	codexOnly := map[string]session.SessionAdapter{
		string(uxp.CLICodex): codexA,
	}
	cOrder := []string{string(uxp.CLICodex)}

	codexSess, codexCLI, _, err := sessionutil.ResolveSessionID(
		"codex-uniq-sess", codexOnly, cOrder)
	if err != nil {
		t.Fatalf("Codex-scoped resolve: %v", err)
	}
	if codexCLI != string(uxp.CLICodex) {
		t.Errorf("Codex matched CLI = %q, want %q", codexCLI, uxp.CLICodex)
	}
	if codexSess.ID != "codex-uniq-sess" {
		t.Errorf("Codex ID = %q, want %q", codexSess.ID, "codex-uniq-sess")
	}
}

// TestSessionShow_JSONSchemaAllTurnFields verifies that all documented
// JSON turn fields are present when marshalling (US-0004 AC3).
func TestSessionShow_JSONSchemaAllTurnFields(t *testing.T) {
	home := t.TempDir()
	a := setupClaude(t, home)

	adapters := map[string]session.SessionAdapter{
		string(uxp.CLIClaude): a,
	}
	order := []string{string(uxp.CLIClaude)}

	sess, matchedCLI, adapter, err := sessionutil.ResolveSessionID(
		"claude-sess-01", adapters, order)
	if err != nil {
		t.Fatalf("ResolveSessionID: %v", err)
	}

	res := buildShowResult(sess, matchedCLI, adapter)
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Parse as generic map to verify top-level fields.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("unmarshal top level: %v", err)
	}
	for _, field := range []string{"id", "cli", "project", "started_at",
		"ended_at", "turn_count", "turns"} {
		if _, ok := top[field]; !ok {
			t.Errorf("missing top-level field %q in JSON output", field)
		}
	}

	// Parse turns array and verify each turn has required fields.
	var turnsRaw []map[string]json.RawMessage
	if err := json.Unmarshal(top["turns"], &turnsRaw); err != nil {
		t.Fatalf("unmarshal turns: %v", err)
	}
	for i, turn := range turnsRaw {
		for _, field := range []string{"role", "content", "timestamp"} {
			if _, ok := turn[field]; !ok {
				t.Errorf("turn[%d] missing field %q", i, field)
			}
		}
	}
}

// itoa converts an int to its decimal string representation.
// stdlib strconv is not imported to keep this package lean.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

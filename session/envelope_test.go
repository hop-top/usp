package session

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSessionJSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	ended := now.Add(30 * time.Minute)
	s := Session{
		ID:         "abc-123",
		CLI:        "claude",
		ProjectCwd: "/Users/test/project",
		StartedAt:  now,
		EndedAt:    &ended,
		TurnCount:  5,
		Metadata:   map[string]any{"branch": "main"},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Session
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != s.ID {
		t.Errorf("ID = %q, want %q", got.ID, s.ID)
	}
	if got.CLI != s.CLI {
		t.Errorf("CLI = %q, want %q", got.CLI, s.CLI)
	}
	if got.TurnCount != s.TurnCount {
		t.Errorf("TurnCount = %d, want %d", got.TurnCount, s.TurnCount)
	}
	if got.EndedAt == nil {
		t.Fatal("EndedAt is nil after round-trip")
	}
}

func TestTurnJSONRoundTrip(t *testing.T) {
	turn := Turn{
		Role:      RoleAssistant,
		Content:   "Hello",
		Timestamp: time.Now().Truncate(time.Second),
		ToolCalls: []ToolCall{
			{Name: "Read", Input: "/tmp/f.txt", Output: "contents"},
		},
	}

	data, err := json.Marshal(turn)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Turn
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Role != RoleAssistant {
		t.Errorf("Role = %q, want %q", got.Role, RoleAssistant)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(got.ToolCalls))
	}
	if got.ToolCalls[0].Name != "Read" {
		t.Errorf("ToolCall.Name = %q, want %q", got.ToolCalls[0].Name, "Read")
	}
}

func TestSessionZeroValue(t *testing.T) {
	var s Session
	if s.EndedAt != nil {
		t.Error("zero Session.EndedAt should be nil")
	}
	if s.TurnCount != 0 {
		t.Error("zero Session.TurnCount should be 0")
	}
	if s.Metadata != nil {
		t.Error("zero Session.Metadata should be nil")
	}
}

func TestTurnSubtypeAndMetadataRoundTrip(t *testing.T) {
	turn := Turn{
		Role:      RoleAssistant,
		Content:   "ok",
		Timestamp: time.Now().Truncate(time.Second),
		Subtype:   "tool-result",
		Metadata: map[string]any{
			"usage.tokens.input":  float64(120),
			"usage.tokens.output": float64(80),
			"assistant.model":     "claude-opus-4-7",
		},
	}

	data, err := json.Marshal(turn)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Turn
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Subtype != "tool-result" {
		t.Errorf("Subtype = %q, want %q", got.Subtype, "tool-result")
	}
	if got.Metadata["assistant.model"] != "claude-opus-4-7" {
		t.Errorf("Metadata[assistant.model] = %v, want claude-opus-4-7", got.Metadata["assistant.model"])
	}
	if got.Metadata["usage.tokens.input"] != float64(120) {
		t.Errorf("Metadata[usage.tokens.input] = %v, want 120", got.Metadata["usage.tokens.input"])
	}
}

func TestTurnOmitsEmptySubtypeAndMetadata(t *testing.T) {
	turn := Turn{
		Role:      RoleUser,
		Content:   "hi",
		Timestamp: time.Now().Truncate(time.Second),
	}

	data, err := json.Marshal(turn)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	s := string(data)
	if strings.Contains(s, `"subtype"`) {
		t.Errorf("empty Subtype should be omitted; got %s", s)
	}
	if strings.Contains(s, `"metadata"`) {
		t.Errorf("nil Metadata should be omitted; got %s", s)
	}
}

func TestTurnPreservesUnknownMetadataKeys(t *testing.T) {
	raw := []byte(`{"role":"assistant","content":"x","timestamp":"2026-04-29T00:00:00Z","metadata":{"custom.key":"v","other":42}}`)

	var got Turn
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Metadata["custom.key"] != "v" {
		t.Errorf("Metadata[custom.key] = %v, want v", got.Metadata["custom.key"])
	}
	if got.Metadata["other"] != float64(42) {
		t.Errorf("Metadata[other] = %v, want 42", got.Metadata["other"])
	}
}

// Package session defines the normalized USP session envelope and
// adapter interface for cross-CLI session management.
package session

import (
	"time"

	"hop.top/kit/uxp"
)

// Session is the normalized USP session envelope representing a
// single coding session from any supported CLI. A session may span
// multiple CLIs via segments — each segment is one CLI's contribution
// to the conversation.
//
// Metadata key namespaces (adapters populate any subset the source
// provides; missing keys remain absent — don't write zero values):
//
//   - usage.tokens.input        (int)     input tokens
//   - usage.tokens.output       (int)     output tokens
//   - usage.tokens.cache_read   (int)     prompt-cache hits
//   - usage.tokens.cache_write  (int)     prompt-cache writes
//   - usage.cost_usd            (float64) total cost in USD
//   - assistant.model           (string)  e.g. "claude-opus-4-7"
//   - performance.duration_ms   (int64)   wall-clock duration
//   - cli_version               (string)  source CLI version
type Session struct {
	ID         string            `json:"id"`
	CLI        uxp.CLIName       `json:"cli"`
	ProjectCwd string            `json:"project_cwd"`
	StartedAt  time.Time         `json:"started_at"`
	EndedAt    *time.Time        `json:"ended_at,omitempty"`
	TurnCount  int               `json:"turn_count"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
	Segments   []Segment         `json:"segments,omitempty"`
	ParentID   string            `json:"parent_id,omitempty"`
}

// Segment records one CLI's contribution to a cross-CLI session.
// A session with no segments was created and completed in one CLI.
// A session with segments was resumed across CLIs via `usp resume`.
type Segment struct {
	CLI       uxp.CLIName `json:"cli"`
	NativeID  string      `json:"native_id"`
	StartedAt time.Time   `json:"started_at"`
	EndedAt   *time.Time  `json:"ended_at,omitempty"`
	TurnCount int         `json:"turn_count"`
}

// Turn represents a single conversational exchange within a session.
//
// Subtype distinguishes non-regular turns: "slash-command",
// "ide-notif", "tool-result", "sidechain". Empty Subtype = regular.
//
// Metadata uses the same key conventions as Session.Metadata when the
// keys are turn-scoped (e.g. usage.tokens.* on assistant turns).
type Turn struct {
	Role      Role           `json:"role"`
	Content   string         `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
	ToolCalls []ToolCall     `json:"tool_calls,omitempty"`
	Subtype   string         `json:"subtype,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// Role identifies the participant in a turn.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// ToolCall captures a tool invocation within an assistant turn.
type ToolCall struct {
	Name   string `json:"name"`
	Input  string `json:"input,omitempty"`
	Output string `json:"output,omitempty"`
	// ID is the tool_use block ID used to correlate with tool_result turns.
	// Populated by adapters; not serialized.
	ID string `json:"-"`
}

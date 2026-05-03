// Package session defines the normalized USP session envelope and
// adapter interface for cross-CLI session management.
package session

import (
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/internal/id"
)

// ID is the canonical TypeID (e.g. sess_…) — the user-facing handle
// for resume/lineage/show. NativeID preserves the underlying CLI
// session identifier (UUIDv4 for Claude, UUIDv7 for Codex, ses_…
// for OpenCode, tag for Gemini) so usp can still locate the
// vendor file on disk. Both forms are accepted everywhere a session
// id is taken — see internal/sessionutil.ResolveSessionID.
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
	ID         string         `json:"id"`
	NativeID   string         `json:"native_id,omitempty"`
	CLI        uxp.CLIName    `json:"cli"`
	ProjectCwd string         `json:"project_cwd"`
	StartedAt  time.Time      `json:"started_at"`
	EndedAt    *time.Time     `json:"ended_at,omitempty"`
	TurnCount  int            `json:"turn_count"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Segments   []Segment      `json:"segments,omitempty"`
	ParentID   string         `json:"parent_id,omitempty"`
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

// SetIDs populates ID with a derived TypeID and NativeID with the
// adapter-supplied native session id. Adapters MUST call this when
// building a Session so resolver lookups work on either form.
//
// On encoding error (e.g. empty native), ID is set to native as a
// fallback — better to surface an unfamiliar id than to drop it.
func (s *Session) SetIDs(native string) {
	s.NativeID = native
	tid, err := id.EncodeFromNative(id.PrefixSession, native)
	if err != nil {
		s.ID = native
		return
	}
	s.ID = tid
}

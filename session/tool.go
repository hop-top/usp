package session

import "time"

// ToolEvent captures one tool call from a session turn, enriched with
// Kit's cross-CLI universal tool taxonomy when available.
type ToolEvent struct {
	SessionID       string    `json:"session_id"`
	NativeSessionID string    `json:"native_session_id,omitempty"`
	CLI             string    `json:"cli"`
	Timestamp       time.Time `json:"ts"`
	TurnID          string    `json:"turn_id,omitempty"`
	Name            string    `json:"name"`
	Universal       string    `json:"universal,omitempty"`
	Label           string    `json:"label"`
	Category        string    `json:"category,omitempty"`
	Input           string    `json:"input,omitempty"`
	Output          string    `json:"output,omitempty"`
}

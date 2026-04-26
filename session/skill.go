package session

import "time"

// SkillOutcome describes what happened to a skill invocation.
type SkillOutcome string

const (
	// SkillInvoked: the skill ran (slash command parsed, Skill
	// tool returned a result, or an equivalent successful path).
	SkillInvoked SkillOutcome = "invoked"
	// SkillDeclined: the model surfaced the skill candidate but
	// did not actually invoke it (no tool call followed).
	SkillDeclined SkillOutcome = "declined"
	// SkillErrored: the invocation was attempted but failed
	// (tool returned is_error or transport-level failure).
	SkillErrored SkillOutcome = "errored"
)

// SkillEvent captures a single skill invocation surfaced by an
// adapter. Every adapter that implements [SkillExtractor] emits
// these per session.
type SkillEvent struct {
	SessionID     string       `json:"session_id"`
	CLI           string       `json:"cli"`
	Timestamp     time.Time    `json:"ts"`
	Name          string       `json:"skill_name"`
	TriggerTurnID string       `json:"trigger_turn_id"`
	TriggerQuery  string       `json:"trigger_query"`
	Outcome       SkillOutcome `json:"outcome"`
	// Unsupported is set when an adapter cannot enumerate skills
	// for the underlying CLI; the row is emitted with empty
	// fields so callers can distinguish "no skills" from
	// "adapter doesn't support it yet".
	Unsupported bool `json:"unsupported,omitempty"`
}

// SkillExtractor is implemented by adapters that can enumerate
// skill invocations from a session. Adapters that lack the
// primitive simply do not implement this interface; the CLI
// layer treats absence as "unsupported".
type SkillExtractor interface {
	// ExtractSkills returns every skill invocation found in the
	// given session, in chronological order. Returns nil (not
	// an error) if the session has no skill activity.
	ExtractSkills(sessionID string) ([]SkillEvent, error)
}

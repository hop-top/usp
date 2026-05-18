package session

import "hop.top/kit/go/core/uxp"

// SessionAdapter extends uxp.Adapter with session-specific operations.
// Each CLI adapter implements this interface.
type SessionAdapter interface {
	uxp.Adapter

	// ListSessions returns all sessions for the given project cwd.
	// Empty cwd returns sessions across all projects.
	ListSessions(cwd string) ([]Session, error)

	// GetSession returns a single session by its native CLI ID.
	GetSession(id string) (*Session, error)

	// StreamTurns returns a channel of turns for the given session.
	// The channel is closed when all turns have been sent or on error.
	StreamTurns(id string) (<-chan Turn, error)

	// ProjectKey returns the CLI-native project key for the given cwd.
	ProjectKey(cwd string) string
}

// ResumeAdapter extends SessionAdapter with the ability to inject
// turns from another CLI's session into this CLI's native format,
// enabling cross-CLI session resume.
type ResumeAdapter interface {
	SessionAdapter

	// InjectSession writes turns into the CLI's native session store,
	// creating a new native session that continues the conversation.
	// Returns the native session ID that can be passed to the CLI's
	// --resume flag.
	InjectSession(cwd string, turns []Turn) (nativeID string, err error)

	// ResumeCmd returns the CLI command + args to resume the injected
	// session. E.g. ["claude", "--resume", "<id>"] or
	// ["codex", "resume", "<id>"].
	ResumeCmd(nativeID string) []string
}

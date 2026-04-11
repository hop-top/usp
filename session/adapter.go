package session

import "hop.top/kit/uxp"

// SessionAdapter extends uxp.Adapter with session-specific operations.
// Each CLI adapter implements this interface.
type SessionAdapter interface {
	uxp.Adapter

	// ListSessions returns all sessions for the given project cwd.
	ListSessions(cwd string) ([]Session, error)

	// GetSession returns a single session by its native CLI ID.
	GetSession(id string) (*Session, error)

	// StreamTurns returns a channel of turns for the given session.
	// The channel is closed when all turns have been sent or on error.
	StreamTurns(id string) (<-chan Turn, error)

	// ProjectKey returns the CLI-native project key for the given cwd.
	ProjectKey(cwd string) string
}

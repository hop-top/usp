// Package lineage tracks cross-CLI session chains. A single logical
// session may span multiple CLIs via segments — each segment is one
// CLI's native session contributing to the conversation.
package lineage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	project_cwd TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS segments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id),
	cli TEXT NOT NULL,
	native_id TEXT NOT NULL,
	started_at INTEGER NOT NULL,
	ended_at INTEGER,
	turn_count INTEGER DEFAULT 0,
	seq INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_segments_session
	ON segments(session_id, seq);
CREATE INDEX IF NOT EXISTS idx_sessions_cwd
	ON sessions(project_cwd);
`

// Store persists cross-CLI session lineage in SQLite.
type Store struct {
	db *sql.DB
}

// Open opens or creates the lineage store at the given path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("lineage: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_journal=WAL")
	if err != nil {
		return nil, fmt.Errorf("lineage: open: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("lineage: migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the store.
func (s *Store) Close() error { return s.db.Close() }

// CreateSession starts a new logical session for the given cwd.
// Returns the USP session ID.
func (s *Store) CreateSession(id, cwd string) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, project_cwd, created_at) VALUES (?, ?, ?)`,
		id, cwd, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("lineage: create session: %w", err)
	}
	return nil
}

// AddSegment records a CLI's contribution to a session.
func (s *Store) AddSegment(
	sessionID string, cli uxp.CLIName, nativeID string,
	turnCount int,
) error {
	var maxSeq int
	row := s.db.QueryRow(
		`SELECT COALESCE(MAX(seq), 0) FROM segments WHERE session_id = ?`,
		sessionID,
	)
	if err := row.Scan(&maxSeq); err != nil {
		return fmt.Errorf("lineage: max seq: %w", err)
	}

	_, err := s.db.Exec(
		`INSERT INTO segments (session_id, cli, native_id, started_at, turn_count, seq)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, cli, nativeID, time.Now().Unix(), turnCount, maxSeq+1,
	)
	if err != nil {
		return fmt.Errorf("lineage: add segment: %w", err)
	}
	return nil
}

// EndSegment marks the latest segment as ended.
func (s *Store) EndSegment(
	sessionID string, turnCount int,
) error {
	_, err := s.db.Exec(
		`UPDATE segments SET ended_at = ?, turn_count = ?
		 WHERE session_id = ? AND ended_at IS NULL
		 ORDER BY seq DESC LIMIT 1`,
		time.Now().Unix(), turnCount, sessionID,
	)
	return err
}

// GetSession returns a session with all its segments.
func (s *Store) GetSession(id string) (*session.Session, error) {
	var cwd string
	var createdAt int64
	err := s.db.QueryRow(
		`SELECT project_cwd, created_at FROM sessions WHERE id = ?`, id,
	).Scan(&cwd, &createdAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("lineage: session %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("lineage: get session: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT cli, native_id, started_at, ended_at, turn_count
		 FROM segments WHERE session_id = ? ORDER BY seq`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("lineage: get segments: %w", err)
	}
	defer rows.Close()

	var segments []session.Segment
	var totalTurns int
	for rows.Next() {
		var (
			cli       string
			nativeID  string
			startedAt int64
			endedAt   sql.NullInt64
			turns     int
		)
		if err := rows.Scan(
			&cli, &nativeID, &startedAt, &endedAt, &turns,
		); err != nil {
			continue
		}
		seg := session.Segment{
			CLI:       cli,
			NativeID:  nativeID,
			StartedAt: time.Unix(startedAt, 0),
			TurnCount: turns,
		}
		if endedAt.Valid {
			t := time.Unix(endedAt.Int64, 0)
			seg.EndedAt = &t
		}
		segments = append(segments, seg)
		totalTurns += turns
	}

	sess := &session.Session{
		ID:         id,
		ProjectCwd: cwd,
		StartedAt:  time.Unix(createdAt, 0),
		TurnCount:  totalTurns,
		Segments:   segments,
	}
	if len(segments) > 0 {
		sess.CLI = segments[0].CLI
	}
	return sess, nil
}

// ListSessions returns all sessions, optionally filtered by cwd.
func (s *Store) ListSessions(cwd string) ([]session.Session, error) {
	var query string
	var args []any
	if cwd == "" {
		query = `SELECT id FROM sessions ORDER BY created_at DESC`
	} else {
		query = `SELECT id FROM sessions WHERE project_cwd = ? ORDER BY created_at DESC`
		args = []any{cwd}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("lineage: list: %w", err)
	}
	defer rows.Close()

	var sessions []session.Session
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		sess, err := s.GetSession(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, *sess)
	}
	return sessions, nil
}

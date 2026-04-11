// Package opencode implements session.SessionAdapter for OpenCode.
package opencode

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"

	_ "modernc.org/sqlite"
)

// Adapter implements session.SessionAdapter for OpenCode.
type Adapter struct {
	registry *uxp.CLIRegistry
	// homeDir overrides os.UserHomeDir for testing.
	homeDir string
	// dbPath overrides the default SQLite path for testing.
	dbPath string
}

// Option configures an Adapter.
type Option func(*Adapter)

// WithHomeDir overrides the home directory (for testing).
func WithHomeDir(dir string) Option {
	return func(a *Adapter) { a.homeDir = dir }
}

// WithDBPath overrides the SQLite database path (for testing).
func WithDBPath(path string) Option {
	return func(a *Adapter) { a.dbPath = path }
}

// New returns a configured OpenCode adapter.
func New(opts ...Option) *Adapter {
	a := &Adapter{registry: uxp.DefaultRegistry()}
	for _, o := range opts {
		o(a)
	}
	return a
}

// CLI returns the canonical CLI name.
func (a *Adapter) CLI() uxp.CLIName { return uxp.CLIOpenCode }

// Detect probes for OpenCode installation.
func (a *Adapter) Detect() (*uxp.DetectResult, error) {
	return uxp.Detect(uxp.CLIOpenCode, a.registry, nil)
}

// Capabilities returns the feature-support map for OpenCode.
func (a *Adapter) Capabilities() uxp.CapabilityMap { return &capMap{} }

// ProjectKey derives the OpenCode project key from cwd.
// Rule: SHA1 hex of absolute cwd (40 chars).
func (a *Adapter) ProjectKey(cwd string) string {
	return uxp.DeriveKey(cwd, uxp.SHA1)
}

// dbFilePath returns the path to opencode.db.
func (a *Adapter) dbFilePath() (string, error) {
	if a.dbPath != "" {
		return a.dbPath, nil
	}
	home := a.homeDir
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(
		home, ".local", "share", "opencode", "opencode.db",
	), nil
}

// openDB opens the SQLite database in read-only mode.
func (a *Adapter) openDB() (*sql.DB, error) {
	path, err := a.dbFilePath()
	if err != nil {
		return nil, err
	}
	return sql.Open("sqlite", path+"?mode=ro")
}

// ListSessions returns all sessions for the given project cwd.
func (a *Adapter) ListSessions(cwd string) ([]session.Session, error) {
	db, err := a.openDB()
	if err != nil {
		return nil, nil // missing DB → empty, not error
	}
	defer db.Close()

	var rows *sql.Rows
	if cwd == "" {
		rows, err = db.Query(`
			SELECT s.id, s.title, s.created_at, s.updated_at,
				COUNT(m.id) AS turn_count
			FROM session s
			LEFT JOIN message m ON m.session_id = s.id
			GROUP BY s.id
			ORDER BY s.created_at DESC`)
	} else {
		projectID := a.ProjectKey(cwd)
		rows, err = db.Query(`
			SELECT s.id, s.title, s.created_at, s.updated_at,
				COUNT(m.id) AS turn_count
			FROM session s
			LEFT JOIN message m ON m.session_id = s.id
			WHERE s.project_id = ?
			GROUP BY s.id
			ORDER BY s.created_at DESC`,
			projectID,
		)
	}
	if err != nil {
		return nil, nil // table might not exist yet
	}
	defer rows.Close()

	var sessions []session.Session
	for rows.Next() {
		var (
			id        string
			title     sql.NullString
			createdAt int64
			updatedAt int64
			turnCount int
		)
		if err := rows.Scan(
			&id, &title, &createdAt, &updatedAt, &turnCount,
		); err != nil {
			continue
		}

		s := session.Session{
			ID:         id,
			CLI:        uxp.CLIOpenCode,
			ProjectCwd: cwd,
			StartedAt:  msToTime(createdAt),
			TurnCount:  turnCount,
			Metadata:   make(map[string]any),
		}
		if title.Valid && title.String != "" {
			s.Metadata["title"] = title.String
		}
		if updatedAt > 0 {
			t := msToTime(updatedAt)
			s.EndedAt = &t
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetSession returns a single session by its native ID.
func (a *Adapter) GetSession(id string) (*session.Session, error) {
	db, err := a.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var (
		projectID string
		title     sql.NullString
		directory sql.NullString
		createdAt int64
		updatedAt int64
	)
	err = db.QueryRow(`
		SELECT project_id, title, directory, created_at, updated_at
		FROM session WHERE id = ?`, id,
	).Scan(&projectID, &title, &directory, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	// Count turns.
	var turnCount int
	_ = db.QueryRow(
		`SELECT COUNT(*) FROM message WHERE session_id = ?`, id,
	).Scan(&turnCount)

	s := &session.Session{
		ID:        id,
		CLI:       uxp.CLIOpenCode,
		StartedAt: msToTime(createdAt),
		TurnCount: turnCount,
		Metadata:  make(map[string]any),
	}
	if directory.Valid {
		s.ProjectCwd = directory.String
	}
	if title.Valid && title.String != "" {
		s.Metadata["title"] = title.String
	}
	if updatedAt > 0 {
		t := msToTime(updatedAt)
		s.EndedAt = &t
	}
	return s, nil
}

// StreamTurns returns a channel of turns for the given session.
func (a *Adapter) StreamTurns(id string) (<-chan session.Turn, error) {
	db, err := a.openDB()
	if err != nil {
		return nil, err
	}

	// Query messages ordered by creation time.
	msgRows, err := db.Query(`
		SELECT id, data, created_at
		FROM message
		WHERE session_id = ?
		ORDER BY created_at ASC`, id,
	)
	if err != nil {
		db.Close()
		return nil, err
	}

	type msgRow struct {
		id        string
		data      string
		createdAt int64
	}
	var msgs []msgRow
	for msgRows.Next() {
		var m msgRow
		if err := msgRows.Scan(&m.id, &m.data, &m.createdAt); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	msgRows.Close()

	ch := make(chan session.Turn)
	go func() {
		defer close(ch)
		defer db.Close()

		for _, m := range msgs {
			turn := msgToTurn(m.data, m.createdAt)

			// Query parts by MESSAGE ID (not session ID).
			partRows, err := db.Query(`
				SELECT data FROM part
				WHERE message_id = ?
				ORDER BY created_at ASC`, m.id,
			)
			if err == nil {
				for partRows.Next() {
					var partData string
					if err := partRows.Scan(&partData); err != nil {
						continue
					}
					appendPartData(&turn, partData)
				}
				partRows.Close()
			}

			ch <- turn
		}
	}()
	return ch, nil
}

// messageData is the JSON structure stored in message.data.
type messageData struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// partDataJSON is the JSON structure stored in part.data.
type partDataJSON struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	ToolName string          `json:"toolName"`
	Input    json.RawMessage `json:"input"`
	Output   string          `json:"output"`
}

// msgToTurn converts a message row to a Turn.
func msgToTurn(data string, createdAt int64) session.Turn {
	turn := session.Turn{
		Timestamp: msToTime(createdAt),
	}

	var md messageData
	if err := json.Unmarshal([]byte(data), &md); err != nil {
		return turn
	}

	switch md.Role {
	case "user":
		turn.Role = session.RoleUser
	case "assistant":
		turn.Role = session.RoleAssistant
	case "system":
		turn.Role = session.RoleSystem
	default:
		turn.Role = session.Role(md.Role)
	}
	turn.Content = md.Content
	return turn
}

// appendPartData enriches a turn with data from a part row.
func appendPartData(turn *session.Turn, data string) {
	var pd partDataJSON
	if err := json.Unmarshal([]byte(data), &pd); err != nil {
		return
	}

	switch pd.Type {
	case "text":
		if pd.Text != "" {
			if turn.Content != "" {
				turn.Content += "\n"
			}
			turn.Content += pd.Text
		}
	case "tool-invocation", "tool_use":
		tc := session.ToolCall{Name: pd.ToolName}
		if len(pd.Input) > 0 {
			tc.Input = string(pd.Input)
		}
		if pd.Output != "" {
			tc.Output = pd.Output
		}
		turn.ToolCalls = append(turn.ToolCalls, tc)
	}
}

// msToTime converts epoch-milliseconds to time.Time.
func msToTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// capMap implements uxp.CapabilityMap for OpenCode.
type capMap struct{}

var capCoverage = map[string]uxp.Support{
	// 16 native
	"session.list":         uxp.Native,
	"session.resume":       uxp.Native,
	"session.transcript":   uxp.Native,
	"session.compaction":   uxp.Native,
	"session.diff":         uxp.Native,
	"session.snapshot":     uxp.Native,
	"project.grouping":     uxp.Native,
	"project.metadata":     uxp.Native,
	"tool.use":             uxp.Native,
	"tool.result":          uxp.Native,
	"permission.mode":      uxp.Native,
	"todo.state":           uxp.Native,
	"workspace.branch":     uxp.Native,
	"event.sourcing":       uxp.Native,
	"plugin.lifecycle":     uxp.Native,
	"file.mirror":          uxp.Native,
	// 2 workaround
	"session.search":       uxp.Workaround,
	"session.export":       uxp.Workaround,
	// 1 missing
	"project.stable.id":    uxp.Missing,
}

func (c *capMap) Supports(dim string) bool {
	s, ok := capCoverage[dim]
	return ok && s != uxp.Missing
}

func (c *capMap) Coverage() map[string]uxp.Support {
	out := make(map[string]uxp.Support, len(capCoverage))
	for k, v := range capCoverage {
		out[k] = v
	}
	return out
}


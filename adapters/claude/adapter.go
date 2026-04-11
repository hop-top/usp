// Package claude implements session.SessionAdapter for Claude Code.
package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// Adapter implements session.SessionAdapter for Claude Code.
type Adapter struct {
	registry *uxp.CLIRegistry
	// homeDir overrides os.UserHomeDir for testing.
	homeDir string
}

// Option configures an Adapter.
type Option func(*Adapter)

// WithHomeDir overrides the home directory (for testing).
func WithHomeDir(dir string) Option {
	return func(a *Adapter) { a.homeDir = dir }
}

// New returns a configured Claude Code adapter.
func New(opts ...Option) *Adapter {
	a := &Adapter{registry: uxp.DefaultRegistry()}
	for _, o := range opts {
		o(a)
	}
	return a
}

// CLI returns the canonical CLI name.
func (a *Adapter) CLI() uxp.CLIName { return uxp.CLIClaude }

// Detect probes for Claude Code installation.
func (a *Adapter) Detect() (*uxp.DetectResult, error) {
	return uxp.Detect(uxp.CLIClaude, a.registry, nil)
}

// Capabilities returns the feature-support map for Claude Code.
func (a *Adapter) Capabilities() uxp.CapabilityMap { return &capMap{} }

// ProjectKey derives the Claude Code project key from cwd.
// Rule: replace / with -, replace . with -.
func (a *Adapter) ProjectKey(cwd string) string {
	key := strings.ReplaceAll(cwd, "/", "-")
	key = strings.ReplaceAll(key, ".", "-")
	return key
}

// storeRoot returns the absolute path to ~/.claude/projects/.
func (a *Adapter) storeRoot() (string, error) {
	home := a.homeDir
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// ListSessions returns sessions for the given cwd. When cwd is empty,
// returns sessions across ALL projects.
func (a *Adapter) ListSessions(cwd string) ([]session.Session, error) {
	root, err := a.storeRoot()
	if err != nil {
		return nil, err
	}

	if cwd == "" {
		return a.listAllProjects(root)
	}

	return a.listProject(root, a.ProjectKey(cwd), cwd)
}

func (a *Adapter) listAllProjects(root string) ([]session.Session, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var all []session.Session
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		sessions, err := a.listProject(root, d.Name(), "")
		if err != nil {
			continue
		}
		all = append(all, sessions...)
	}
	return all, nil
}

func (a *Adapter) listProject(root, key, cwd string) ([]session.Session, error) {
	projDir := filepath.Join(root, key)
	entries, err := os.ReadDir(projDir)
	if err != nil {
		return nil, err
	}

	var sessions []session.Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		s, err := a.parseSession(filepath.Join(projDir, e.Name()), id, cwd)
		if err != nil {
			continue
		}
		sessions = append(sessions, *s)
	}
	return sessions, nil
}

// GetSession returns a single session parsed from its JSONL file.
func (a *Adapter) GetSession(id string) (*session.Session, error) {
	path, err := a.findSessionFile(id)
	if err != nil {
		return nil, err
	}
	return a.parseSession(path, id, "")
}

// StreamTurns reads the JSONL file line by line and emits turns.
func (a *Adapter) StreamTurns(id string) (<-chan session.Turn, error) {
	path, err := a.findSessionFile(id)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	ch := make(chan session.Turn)
	go func() {
		defer close(ch)
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			turn, ok := eventToTurn(scanner.Bytes())
			if ok {
				ch <- turn
			}
		}
	}()
	return ch, nil
}

// findSessionFile walks all project dirs looking for <id>.jsonl.
func (a *Adapter) findSessionFile(id string) (string, error) {
	root, err := a.storeRoot()
	if err != nil {
		return "", err
	}

	dirs, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}

	target := id + ".jsonl"
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		p := filepath.Join(root, d.Name(), target)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

// jsonlEvent is the raw JSONL schema from Claude Code transcripts.
type jsonlEvent struct {
	UUID       string          `json:"uuid"`
	ParentUUID string          `json:"parentUuid"`
	Type       string          `json:"type"`
	Timestamp  string          `json:"timestamp"`
	SessionID  string          `json:"sessionId"`
	CWD        string          `json:"cwd"`
	GitBranch  string          `json:"gitBranch"`
	Message    json.RawMessage `json:"message"`
}

type messagePayload struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// parseSession reads a JSONL file and builds a Session envelope.
// If cwd is provided, it seeds ProjectCwd (otherwise read from events).
func (a *Adapter) parseSession(
	path, id, cwd string,
) (*session.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := &session.Session{
		ID:         id,
		CLI:        uxp.CLIClaude,
		ProjectCwd: cwd,
		Metadata:   make(map[string]any),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var lastTS time.Time

	for scanner.Scan() {
		var ev jsonlEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		ts := parseTimestamp(ev.Timestamp)
		if s.StartedAt.IsZero() && !ts.IsZero() {
			s.StartedAt = ts
		}
		if !ts.IsZero() {
			lastTS = ts
		}
		if ev.CWD != "" && s.ProjectCwd == "" {
			s.ProjectCwd = ev.CWD
		}
		if ev.GitBranch != "" {
			s.Metadata["gitBranch"] = ev.GitBranch
		}
		if ev.SessionID != "" {
			s.Metadata["sessionId"] = ev.SessionID
		}
		if ev.Type == "user" || ev.Type == "assistant" {
			s.TurnCount++
		}
	}
	if !lastTS.IsZero() {
		s.EndedAt = &lastTS
	}
	return s, nil
}

// eventToTurn converts a raw JSONL line to a session.Turn.
// Returns false if the event is not a conversation turn.
func eventToTurn(line []byte) (session.Turn, bool) {
	var ev jsonlEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return session.Turn{}, false
	}

	var role session.Role
	switch ev.Type {
	case "user":
		role = session.RoleUser
	case "assistant":
		role = session.RoleAssistant
	case "system":
		role = session.RoleSystem
	default:
		return session.Turn{}, false
	}

	turn := session.Turn{
		Role:      role,
		Timestamp: parseTimestamp(ev.Timestamp),
	}

	// Extract content from message payload.
	if len(ev.Message) > 0 {
		var msg messagePayload
		if err := json.Unmarshal(ev.Message, &msg); err == nil {
			turn.Content = extractContent(msg.Content)
			if role == session.RoleAssistant {
				turn.ToolCalls = extractToolCalls(msg.Content)
			}
		}
	}

	return turn, true
}

// extractContent pulls text from message content (string or array).
func extractContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try plain string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try content blocks array.
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}

	return ""
}

// extractToolCalls pulls tool_use blocks from message content.
func extractToolCalls(raw json.RawMessage) []session.ToolCall {
	if len(raw) == 0 {
		return nil
	}

	var blocks []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}

	var calls []session.ToolCall
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		tc := session.ToolCall{Name: b.Name}
		if len(b.Input) > 0 {
			tc.Input = string(b.Input)
		}
		calls = append(calls, tc)
	}
	return calls
}

func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}

type capMap struct{}

var capCoverage = map[string]uxp.Support{
	"session.list": uxp.Native, "session.resume": uxp.Native,
	"session.transcript": uxp.Native, "session.compaction": uxp.Native,
	"session.subagents": uxp.Native, "project.grouping": uxp.Native,
	"project.memory": uxp.Native, "tool.use": uxp.Native,
	"tool.result": uxp.Native, "permission.mode": uxp.Native,
	"git.branch.tracking": uxp.Native, "file.history": uxp.Native,
	"todo.state": uxp.Native, "hook.lifecycle": uxp.Native,
	"session.search": uxp.Workaround, "session.export": uxp.Workaround,
	"session.cross.project": uxp.Missing, "session.relink": uxp.Missing,
	"session.rotation": uxp.Missing, "project.stable.id": uxp.Missing,
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

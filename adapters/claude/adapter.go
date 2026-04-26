// Package claude implements session.SessionAdapter for Claude Code.
package claude

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
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

		// pending holds assistant turns with unresolved tool calls.
		// Tool_use ID → index into pending + ToolCalls for O(1) lookup.
		var pending []session.Turn
		idIdx := make(map[string][2]int) // tool_use_id → [pendingIdx, tcIdx]

		flushAll := func() {
			for _, t := range pending {
				ch <- t
			}
			pending = pending[:0]
			clear(idIdx)
		}

		for scanner.Scan() {
			line := scanner.Bytes()
			turn, ok := eventToTurn(line)
			if !ok {
				continue
			}

			// Try to resolve tool_result user turns into pending assistants.
			if len(pending) > 0 && turn.Role == session.RoleUser {
				if results := extractToolResultsFromLine(line); results != nil {
					for useID, out := range results {
						if idx, found := idIdx[useID]; found {
							pending[idx[0]].ToolCalls[idx[1]].Output = out
						}
					}
					// Flush any fully-resolved pending turns (in order).
					for len(pending) > 0 && allResolved(pending[0]) {
						ch <- pending[0]
						// Clean up index entries for flushed turn.
						for _, tc := range pending[0].ToolCalls {
							delete(idIdx, tc.ID)
						}
						pending = pending[1:]
						// Adjust remaining index entries.
						for id, idx := range idIdx {
							idIdx[id] = [2]int{idx[0] - 1, idx[1]}
						}
					}
					ch <- turn
					continue
				}
			}

			// Another assistant with tool calls: accumulate.
			if turn.Role == session.RoleAssistant && hasUnresolvedToolCalls(turn) {
				pi := len(pending)
				pending = append(pending, turn)
				for ti, tc := range turn.ToolCalls {
					if tc.ID != "" {
						idIdx[tc.ID] = [2]int{pi, ti}
					}
				}
			} else {
				// Non-tool turn: flush all pending first.
				flushAll()
				ch <- turn
			}
		}
		flushAll()
	}()
	return ch, nil
}

// InjectSession writes turns into Claude Code's native JSONL format,
// creating a new session that can be resumed with --resume.
func (a *Adapter) InjectSession(cwd string, turns []session.Turn) (string, error) {
	root, err := a.storeRoot()
	if err != nil {
		return "", err
	}

	key := a.ProjectKey(cwd)
	projDir := filepath.Join(root, key)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		return "", err
	}

	uuid, err := newUUID()
	if err != nil {
		return "", err
	}

	f, err := os.Create(filepath.Join(projDir, uuid+".jsonl"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	enc := json.NewEncoder(f)

	// Summary line.
	if err := enc.Encode(map[string]any{
		"type": "summary", "version": "1",
		"uuid": uuid, "parentUuid": "", "cwd": cwd,
	}); err != nil {
		return "", err
	}

	for _, t := range turns {
		content := t.Content
		if t.Role == session.RoleAssistant && len(t.ToolCalls) > 0 {
			for _, tc := range t.ToolCalls {
				content += fmt.Sprintf("\n[Tool: %s %s → %s]", tc.Name, tc.Input, tc.Output)
			}
		}
		ev := map[string]any{
			"type":      string(t.Role),
			"timestamp": t.Timestamp.Format(time.RFC3339),
			"message": map[string]any{
				"role":    string(t.Role),
				"content": content,
			},
		}
		if err := enc.Encode(ev); err != nil {
			return "", err
		}
	}
	return uuid, nil
}

// ResumeCmd returns the CLI command to resume an injected session.
func (a *Adapter) ResumeCmd(nativeID string) []string {
	return []string{"claude", "--resume", nativeID}
}

// newUUID generates a v4 UUID using crypto/rand.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
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
		CLI:        uxp.CLIClaude,
		ProjectCwd: cwd,
		Metadata:   make(map[string]any),
	}
	s.SetIDs(id)

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
		Type    string          `json:"type"`
		Text    string          `json:"text"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text != "" {
					parts = append(parts, b.Text)
				}
			case "tool_result":
				if s := extractToolResultContent(b.Content); s != "" {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "\n")
	}

	return ""
}

// extractToolResultContent extracts text from a tool_result's content
// field, which can be a plain string or an array of content blocks.
func extractToolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
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
// The ID field is populated for later tool_result correlation.
func extractToolCalls(raw json.RawMessage) []session.ToolCall {
	if len(raw) == 0 {
		return nil
	}

	var blocks []struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
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
		tc := session.ToolCall{ID: b.ID, Name: b.Name}
		if len(b.Input) > 0 {
			tc.Input = string(b.Input)
		}
		calls = append(calls, tc)
	}
	return calls
}

// allResolved reports whether every tool call in the turn has output.
func allResolved(t session.Turn) bool {
	for _, tc := range t.ToolCalls {
		if tc.ID != "" && tc.Output == "" {
			return false
		}
	}
	return true
}

// hasUnresolvedToolCalls reports whether any tool call has an ID
// (meaning it expects a future tool_result resolution).
func hasUnresolvedToolCalls(t session.Turn) bool {
	for _, tc := range t.ToolCalls {
		if tc.ID != "" {
			return true
		}
	}
	return false
}

// extractToolResultsFromLine parses a raw JSONL line and returns the
// tool_use_id → output map if the line is a user turn with tool_result
// content blocks. Returns nil if not applicable.
func extractToolResultsFromLine(line []byte) map[string]string {
	var ev jsonlEvent
	if err := json.Unmarshal(line, &ev); err != nil || ev.Type != "user" {
		return nil
	}
	var msg messagePayload
	if err := json.Unmarshal(ev.Message, &msg); err != nil {
		return nil
	}
	return extractToolResults(msg.Content)
}

// extractToolResults returns a map of tool_use_id → output text
// from a user turn's content blocks containing tool_result entries.
func extractToolResults(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var blocks []struct {
		Type      string          `json:"type"`
		ToolUseID string          `json:"tool_use_id"`
		Content   json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}
	results := make(map[string]string)
	for _, b := range blocks {
		if b.Type != "tool_result" || b.ToolUseID == "" {
			continue
		}
		results[b.ToolUseID] = extractToolResultContent(b.Content)
	}
	if len(results) == 0 {
		return nil
	}
	return results
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

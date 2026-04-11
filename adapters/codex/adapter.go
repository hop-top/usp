// Package codex implements session.SessionAdapter for the Codex CLI.
//
// Codex stores sessions as date-partitioned JSONL under
// ~/.codex/sessions/<YYYY>/<MM>/<DD>/. Project grouping requires
// reading the first-line session_meta header from each file.
package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// Adapter implements session.SessionAdapter for the Codex CLI.
type Adapter struct{}

var _ session.SessionAdapter = (*Adapter)(nil)

func (a *Adapter) CLI() uxp.CLIName { return uxp.CLICodex }

func (a *Adapter) Detect() (*uxp.DetectResult, error) {
	return uxp.Detect(uxp.CLICodex, uxp.DefaultRegistry(), nil)
}

func (a *Adapter) Capabilities() uxp.CapabilityMap { return &capMap{} }

// ProjectKey returns cwd as-is (Embedded strategy).
func (a *Adapter) ProjectKey(cwd string) string {
	return uxp.DeriveKey(cwd, uxp.Embedded)
}

// ListSessions tries session_index.jsonl first, falls back to dir walk.
func (a *Adapter) ListSessions(cwd string) ([]session.Session, error) {
	root, err := sessionsRootFn()
	if err != nil {
		return nil, err
	}
	if s, err := a.listFromIndex(root, cwd); err == nil {
		return s, nil
	}
	return a.listFromWalk(root, cwd)
}

func (a *Adapter) GetSession(id string) (*session.Session, error) {
	root, err := sessionsRootFn()
	if err != nil {
		return nil, err
	}
	path, err := findSessionFile(root, id)
	if err != nil {
		return nil, err
	}
	meta, n, err := readSessionFile(path)
	if err != nil {
		return nil, err
	}
	return metaToSession(meta, n), nil
}

func (a *Adapter) StreamTurns(id string) (<-chan session.Turn, error) {
	root, err := sessionsRootFn()
	if err != nil {
		return nil, err
	}
	path, err := findSessionFile(root, id)
	if err != nil {
		return nil, err
	}
	ch := make(chan session.Turn)
	go func() {
		defer close(ch)
		streamFile(path, ch)
	}()
	return ch, nil
}

// --- types ---

type sessionMeta struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		ID               string `json:"id"`
		Timestamp        string `json:"timestamp"`
		CWD              string `json:"cwd"`
		Originator       string `json:"originator"`
		CLIVersion       string `json:"cli_version"`
		Source           string `json:"source"`
		Instructions     *any   `json:"instructions"`
		BaseInstructions *struct {
			Text string `json:"text"`
		} `json:"base_instructions"`
	} `json:"payload"`
}

type responseItem struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type itemPayload struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type indexEntry struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	UpdatedAt  string `json:"updated_at"`
}

// --- path helpers (overridable for testing) ---

var (
	sessionsRootFn = defaultSessionsRoot
	codexRootFn    = defaultCodexRoot
)

// SetSessionsRoot overrides the sessions root path for testing.
// Returns a restore function.
func SetSessionsRoot(dir string) func() {
	orig := sessionsRootFn
	sessionsRootFn = func() (string, error) { return dir, nil }
	return func() { sessionsRootFn = orig }
}

// SetCodexRoot overrides the codex root path for testing.
// Returns a restore function.
func SetCodexRoot(dir string) func() {
	orig := codexRootFn
	codexRootFn = func() (string, error) { return dir, nil }
	return func() { codexRootFn = orig }
}

func defaultSessionsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("codex: %w", err)
	}
	return filepath.Join(home, ".codex", "sessions"), nil
}

func defaultCodexRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("codex: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

// --- list strategies ---

func (a *Adapter) listFromIndex(root, cwd string) ([]session.Session, error) {
	codex, err := codexRootFn()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(codex, "session_index.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var idx []indexEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e indexEntry
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.ID != "" {
			idx = append(idx, e)
		}
	}
	if len(idx) == 0 {
		return nil, fmt.Errorf("codex: empty index")
	}

	var out []session.Session
	for _, entry := range idx {
		path, err := findSessionFile(root, entry.ID)
		if err != nil {
			continue
		}
		meta, n, err := readSessionFile(path)
		if err != nil {
			continue
		}
		if cwd != "" && meta.Payload.CWD != cwd {
			continue
		}
		s := metaToSession(meta, n)
		if entry.ThreadName != "" {
			s.Metadata["thread_name"] = entry.ThreadName
		}
		out = append(out, *s)
	}
	return out, nil
}

func (a *Adapter) listFromWalk(root, cwd string) ([]session.Session, error) {
	var out []session.Session
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		meta, n, err := readSessionFile(p)
		if err != nil {
			return nil
		}
		if cwd != "" && meta.Payload.CWD != cwd {
			return nil
		}
		out = append(out, *metaToSession(meta, n))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("codex: walk: %w", err)
	}
	return out, nil
}

// --- file helpers ---

func findSessionFile(root, id string) (string, error) {
	var found string
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if strings.Contains(filepath.Base(p), id) {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("codex: session %s not found", id)
	}
	return found, nil
}

func readSessionFile(path string) (*sessionMeta, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return nil, 0, fmt.Errorf("codex: empty file %s", path)
	}
	var meta sessionMeta
	if err := json.Unmarshal(sc.Bytes(), &meta); err != nil {
		return nil, 0, fmt.Errorf("codex: parse meta %s: %w", path, err)
	}
	if meta.Type != "session_meta" {
		return nil, 0, fmt.Errorf("codex: unexpected type %q in %s", meta.Type, path)
	}
	lines := 1
	for sc.Scan() {
		lines++
	}
	return &meta, lines, nil
}

func metaToSession(meta *sessionMeta, lineCount int) *session.Session {
	s := &session.Session{
		ID: meta.Payload.ID, CLI: uxp.CLICodex,
		ProjectCwd: meta.Payload.CWD,
		TurnCount:  lineCount - 1,
		Metadata:   make(map[string]any),
	}
	if t, err := time.Parse(time.RFC3339Nano, meta.Payload.Timestamp); err == nil {
		s.StartedAt = t
	} else if t, err := time.Parse(time.RFC3339Nano, meta.Timestamp); err == nil {
		s.StartedAt = t
	}
	if meta.Payload.Originator != "" {
		s.Metadata["originator"] = meta.Payload.Originator
	}
	if meta.Payload.Source != "" {
		s.Metadata["source"] = meta.Payload.Source
	}
	if meta.Payload.CLIVersion != "" {
		s.Metadata["cli_version"] = meta.Payload.CLIVersion
	}
	return s
}

// --- turn parsing (handles schema drift) ---

func streamFile(path string, ch chan<- session.Turn) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // skip session_meta
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var item responseItem
		if json.Unmarshal(sc.Bytes(), &item) != nil {
			continue
		}
		if turn, ok := parseTurn(item); ok {
			ch <- turn
		}
	}
}

func parseTurn(item responseItem) (session.Turn, bool) {
	ts, _ := time.Parse(time.RFC3339Nano, item.Timestamp)
	var p itemPayload
	if json.Unmarshal(item.Payload, &p) != nil {
		return session.Turn{}, false
	}
	// Works for both "response_item" (v0.106+) and older formats
	// as long as payload has role+content.
	if p.Role == "" {
		return session.Turn{}, false
	}
	role := mapRole(p.Role)
	if role == "" {
		return session.Turn{}, false
	}
	return session.Turn{
		Role: role, Content: extractText(p.Content), Timestamp: ts,
	}, true
}

func mapRole(r string) session.Role {
	switch r {
	case "user":
		return session.RoleUser
	case "assistant":
		return session.RoleAssistant
	case "system":
		return session.RoleSystem
	}
	return ""
}

func extractText(raw json.RawMessage) string {
	var parts []contentPart
	if json.Unmarshal(raw, &parts) == nil && len(parts) > 0 {
		var b strings.Builder
		for i, p := range parts {
			if p.Text == "" {
				continue
			}
			if i > 0 && b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(p.Text)
		}
		return b.String()
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

// --- capability map ---

type capMap struct{}

func (c *capMap) Supports(dim string) bool {
	s, ok := c.Coverage()[dim]
	return ok && s != uxp.Missing
}

func (c *capMap) Coverage() map[string]uxp.Support {
	return map[string]uxp.Support{
		"session_store": uxp.Native, "session_resume": uxp.Native,
		"tool_execution": uxp.Native, "model_selection": uxp.Native,
		"sandbox_mode": uxp.Native,
		"project_grouping": uxp.Workaround, "session_search": uxp.Workaround,
		"prompt_history": uxp.Workaround, "thread_naming": uxp.Workaround,
		"session_archival": uxp.Workaround,
		"project_key": uxp.Missing, "session_branching": uxp.Missing,
		"session_export": uxp.Missing, "session_diff": uxp.Missing,
		"multi_model": uxp.Missing, "cost_tracking": uxp.Missing,
		"token_counting": uxp.Missing, "context_management": uxp.Missing,
		"memory_system": uxp.Missing, "plugin_system": uxp.Missing,
	}
}

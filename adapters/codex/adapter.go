// Package codex implements session.SessionAdapter for the Codex CLI.
//
// Codex stores sessions as date-partitioned JSONL under
// ~/.codex/sessions/<YYYY>/<MM>/<DD>/. Project grouping requires
// reading the first-line session_meta header from each file.
package codex

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hop.top/kit/go/core/uxp"
	kitcodex "hop.top/kit/go/core/uxp/invoke/adapters/codex"
	"hop.top/usp/session"
)

// Adapter implements session.SessionAdapter and session.ResumeAdapter
// for the Codex CLI.
type Adapter struct{}

var _ session.SessionAdapter = (*Adapter)(nil)
var _ session.ResumeAdapter = (*Adapter)(nil)

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
	indexed, indexErr := a.listFromIndex(root, cwd)
	walked, walkErr := a.listFromWalk(root, cwd)
	if walkErr != nil {
		if indexErr == nil {
			return indexed, nil
		}
		return nil, walkErr
	}
	if indexErr != nil {
		return walked, nil
	}
	return mergeSessions(indexed, walked), nil
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
	meta, scan, err := readSessionFile(path)
	if err != nil {
		return nil, err
	}
	return metaToSession(meta, scan), nil
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

// InjectSession writes turns into the Codex native JSONL session
// store, creating a synthetic session that can be resumed.
func (a *Adapter) InjectSession(cwd string, turns []session.Turn) (string, error) {
	root, err := sessionsRootFn()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	id := generateUUIDv7(now)
	dayDir := filepath.Join(root, now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return "", fmt.Errorf("codex: mkdir: %w", err)
	}
	ts := now.Format("2006-01-02T15-04-05")
	fname := fmt.Sprintf("rollout-%s-%s.jsonl", ts, id)
	f, err := os.Create(filepath.Join(dayDir, fname))
	if err != nil {
		return "", fmt.Errorf("codex: create: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	iso := now.Format(time.RFC3339Nano)
	meta := map[string]any{
		"timestamp": iso, "type": "session_meta",
		"payload": map[string]any{
			"id": id, "timestamp": iso, "cwd": cwd,
			"originator": "usp", "cli_version": "0.0.0", "source": "usp",
		},
	}
	if err := enc.Encode(meta); err != nil {
		return "", fmt.Errorf("codex: write meta: %w", err)
	}
	for _, t := range turns {
		content := t.Content
		if len(t.ToolCalls) > 0 {
			var b strings.Builder
			if content != "" {
				b.WriteString(content)
				b.WriteByte('\n')
			}
			for _, tc := range t.ToolCalls {
				fmt.Fprintf(&b, "[Tool: %s", tc.Name)
				if tc.Output != "" {
					fmt.Fprintf(&b, " → %s", tc.Output)
				}
				b.WriteByte(']')
			}
			content = b.String()
		}
		item := map[string]any{
			"timestamp": iso, "type": "response_item",
			"payload": map[string]any{
				"id": "msg_" + randHex(12), "type": "message",
				"role": string(t.Role),
				"content": []map[string]any{
					{"type": "output_text", "text": content},
				},
			},
		}
		if err := enc.Encode(item); err != nil {
			return "", fmt.Errorf("codex: write turn: %w", err)
		}
	}
	return id, nil
}

// ResumeCmd returns the CLI command to resume the given session.
// Delegates to kit's invocation facade so the argv stays in lockstep
// with the cross-CLI matrix in go/core/uxp/README.md. Codex defaults
// to interactive `codex resume <id>` when no output format is
// requested, matching the typical human-in-the-loop resume flow.
func (a *Adapter) ResumeCmd(nativeID string) []string {
	return session.ResumeCmdFor(kitcodex.New(), nativeID)
}

// generateUUIDv7 produces a UUIDv7-style string from the given time
// plus crypto/rand entropy.
func generateUUIDv7(t time.Time) string {
	var b [16]byte
	ms := uint64(t.UnixMilli())
	binary.BigEndian.PutUint32(b[0:4], uint32(ms>>16))
	binary.BigEndian.PutUint16(b[4:6], uint16(ms&0xFFFF))
	_, _ = rand.Read(b[6:])
	b[6] = (b[6] & 0x0F) | 0x70 // version 7
	b[8] = (b[8] & 0x3F) | 0x80 // variant 10
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
		meta, scan, err := readSessionFile(path)
		if err != nil {
			continue
		}
		if cwd != "" && meta.Payload.CWD != cwd {
			continue
		}
		s := metaToSession(meta, scan)
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
		meta, scan, err := readSessionFile(p)
		if err != nil {
			return nil
		}
		if cwd != "" && meta.Payload.CWD != cwd {
			return nil
		}
		out = append(out, *metaToSession(meta, scan))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("codex: walk: %w", err)
	}
	return out, nil
}

func mergeSessions(indexed, walked []session.Session) []session.Session {
	out := make([]session.Session, 0, len(indexed)+len(walked))
	seen := make(map[string]bool, len(indexed)+len(walked))
	for _, s := range indexed {
		key := s.NativeID
		if key == "" {
			key = s.ID
		}
		seen[key] = true
		out = append(out, s)
	}
	for _, s := range walked {
		key := s.NativeID
		if key == "" {
			key = s.ID
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
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

// fileScan holds derived signals from a full pass over the JSONL.
type fileScan struct {
	lineCount int
	model     string // last seen turn_context.payload.model
}

func readSessionFile(path string) (*sessionMeta, fileScan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fileScan{}, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	if !sc.Scan() {
		return nil, fileScan{}, fmt.Errorf("codex: empty file %s", path)
	}
	var meta sessionMeta
	if err := json.Unmarshal(sc.Bytes(), &meta); err != nil {
		return nil, fileScan{}, fmt.Errorf("codex: parse meta %s: %w", path, err)
	}
	if meta.Type != "session_meta" {
		return nil, fileScan{}, fmt.Errorf("codex: unexpected type %q in %s", meta.Type, path)
	}
	scan := fileScan{lineCount: 1}
	for sc.Scan() {
		scan.lineCount++
		// Look for turn_context events carrying model selection.
		if m := extractTurnContextModel(sc.Bytes()); m != "" {
			scan.model = m
		}
	}
	return &meta, scan, nil
}

// extractTurnContextModel pulls payload.model from a turn_context line.
// Returns empty string when the line is not a turn_context event.
func extractTurnContextModel(line []byte) string {
	var ev struct {
		Type    string `json:"type"`
		Payload struct {
			Model string `json:"model"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &ev); err != nil {
		return ""
	}
	if ev.Type != "turn_context" {
		return ""
	}
	return ev.Payload.Model
}

func metaToSession(meta *sessionMeta, scan fileScan) *session.Session {
	s := &session.Session{
		CLI:        uxp.CLICodex,
		ProjectCwd: meta.Payload.CWD,
		TurnCount:  scan.lineCount - 1,
		Metadata:   make(map[string]any),
	}
	s.SetIDs(meta.Payload.ID)
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
	if scan.model != "" {
		s.Metadata["assistant.model"] = scan.model
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
		"sandbox_mode":     uxp.Native,
		"project_grouping": uxp.Workaround, "session_search": uxp.Workaround,
		"prompt_history": uxp.Workaround, "thread_naming": uxp.Workaround,
		"session_archival": uxp.Workaround,
		"project_key":      uxp.Missing, "session_branching": uxp.Missing,
		"session_export": uxp.Missing, "session_diff": uxp.Missing,
		"multi_model": uxp.Missing, "cost_tracking": uxp.Missing,
		"token_counting": uxp.Missing, "context_management": uxp.Missing,
		"memory_system": uxp.Missing, "plugin_system": uxp.Missing,
	}
}

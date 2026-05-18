// Package gemini implements session.SessionAdapter for Gemini CLI.
//
// Store layout:
//
//	~/.gemini/projects.json   — cwd → alias map
//	~/.gemini/tmp/<alias>/chats/session-*.jsonl — per-project chats
package gemini

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hop.top/kit/go/core/uxp"
	kitgemini "hop.top/kit/go/core/uxp/invoke/adapters/gemini"
	"hop.top/usp/session"
)

// projectsFile is the central cwd→alias map inside ~/.gemini/.
const projectsFile = "projects.json"

// projectRootMarker is the sentinel file inside each tmp project dir.
const projectRootMarker = ".project_root"

// projectsJSON mirrors the on-disk schema of ~/.gemini/projects.json.
type projectsJSON struct {
	Projects map[string]string `json:"projects"` // abs-cwd → alias
}

// Adapter implements session.SessionAdapter for Gemini CLI.
type Adapter struct {
	// HomeDir overrides os.UserHomeDir for testing. Empty uses real home.
	HomeDir string
}

// Ensure Adapter satisfies both interfaces at compile time.
var _ session.SessionAdapter = (*Adapter)(nil)
var _ session.ResumeAdapter = (*Adapter)(nil)

// CLI returns the canonical CLI name.
func (a *Adapter) CLI() uxp.CLIName { return uxp.CLIGemini }

// Detect probes the local environment for the Gemini CLI.
func (a *Adapter) Detect() (*uxp.DetectResult, error) {
	return uxp.Detect(uxp.CLIGemini, uxp.DefaultRegistry(), nil)
}

// Capabilities returns the feature-support map for Gemini CLI.
// Counts per the verified dossier: 9 native, 4 workaround, 7 missing.
func (a *Adapter) Capabilities() uxp.CapabilityMap {
	return &capMap{}
}

// ProjectKey returns the Gemini alias for the given cwd by looking it
// up in ~/.gemini/projects.json. Falls back to basename if the map
// cannot be read or the cwd is not registered.
func (a *Adapter) ProjectKey(cwd string) string {
	pm, err := a.loadProjects()
	if err != nil {
		return filepath.Base(cwd)
	}
	if alias, ok := pm.Projects[cwd]; ok {
		return alias
	}
	return filepath.Base(cwd)
}

// ListSessions returns sessions found under Gemini's tmp chat dir for cwd.
func (a *Adapter) ListSessions(cwd string) ([]session.Session, error) {
	if cwd == "" {
		return a.listAllProjectChats()
	}

	alias := a.ProjectKey(cwd)
	return a.listAliasChats(alias, cwd)
}

func (a *Adapter) listAllProjectChats() ([]session.Session, error) {
	tmpRoot := filepath.Join(a.geminiRoot(), "tmp")
	dirs, err := os.ReadDir(tmpRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var all []session.Session
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		cwd := a.projectRootForAlias(d.Name())
		sessions, _ := a.listAliasChats(d.Name(), cwd)
		all = append(all, sessions...)
	}
	return all, nil
}

func (a *Adapter) listAliasChats(alias, cwd string) ([]session.Session, error) {
	chatDir := filepath.Join(a.geminiRoot(), "tmp", alias, "chats")
	entries, err := os.ReadDir(chatDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("gemini: read chat dir: %w", err)
	}

	var sessions []session.Session
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		info, _ := e.Info()
		meta, err := readChatHeader(filepath.Join(chatDir, name))
		if err != nil || meta.SessionID == "" {
			continue
		}
		s := session.Session{
			CLI:        uxp.CLIGemini,
			ProjectCwd: cwd,
			TurnCount:  0,
		}
		s.SetIDs(meta.SessionID)
		if t, err := time.Parse(time.RFC3339Nano, meta.StartTime); err == nil {
			s.StartedAt = t
		} else if info != nil {
			s.StartedAt = info.ModTime()
		}
		if model := readModelFromTranscript(
			filepath.Join(chatDir, name)); model != "" {
			s.Metadata = map[string]any{"assistant.model": model}
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetSession returns a session by its Gemini session UUID.
func (a *Adapter) GetSession(id string) (*session.Session, error) {
	chatPath, cwd, err := a.findChatFile(id)
	if err != nil {
		return nil, err
	}

	info, _ := os.Stat(chatPath)
	meta, _ := readChatHeader(chatPath)
	s := &session.Session{
		CLI:        uxp.CLIGemini,
		ProjectCwd: cwd,
		Metadata:   map[string]any{},
	}
	s.SetIDs(id)
	if t, err := time.Parse(time.RFC3339Nano, meta.StartTime); err == nil {
		s.StartedAt = t
	} else if info != nil {
		s.StartedAt = info.ModTime()
	}
	if model := readModelFromTranscript(chatPath); model != "" {
		s.Metadata["assistant.model"] = model
	}
	if len(s.Metadata) == 0 {
		s.Metadata = nil
	}
	return s, nil
}

// StreamTurns reads Gemini's JSONL chat for the given session and emits one
// Turn per message entry.
func (a *Adapter) StreamTurns(id string) (<-chan session.Turn, error) {
	chatPath, _, err := a.findChatFile(id)
	if err != nil {
		return nil, err
	}

	lines, err := readJSONLines(chatPath)
	if err != nil {
		return nil, fmt.Errorf("gemini: read transcript: %w", err)
	}

	turns := make([]session.Turn, 0, len(lines))
	for _, line := range lines {
		turn, ok := chatLineToTurn(line)
		if ok {
			turns = append(turns, turn)
		}
	}

	ch := make(chan session.Turn, len(turns))
	for _, turn := range turns {
		ch <- turn
	}
	close(ch)
	return ch, nil
}

type chatHeader struct {
	SessionID   string `json:"sessionId"`
	ProjectHash string `json:"projectHash"`
	StartTime   string `json:"startTime"`
	LastUpdated string `json:"lastUpdated"`
	Kind        string `json:"kind"`
}

type chatContentPart struct {
	Text string `json:"text"`
}

// InjectSession writes turns into Gemini's native JSONL chat store,
// creating a UUID-backed session that can be resumed via `gemini --resume
// <uuid>`.
func (a *Adapter) InjectSession(cwd string, turns []session.Turn) (string, error) {
	alias := a.ProjectKey(cwd)
	projectDir := filepath.Join(a.geminiRoot(), "tmp", alias)
	chatDir := filepath.Join(projectDir, "chats")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		return "", fmt.Errorf("gemini: create chat dir: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(projectDir, projectRootMarker), []byte(cwd), 0o644,
	); err != nil {
		return "", fmt.Errorf("gemini: write project root marker: %w", err)
	}

	sessionID, err := generateUUID()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)

	var lines []any
	lines = append(lines, chatHeader{
		SessionID:   sessionID,
		ProjectHash: projectHash(cwd),
		StartTime:   nowStr,
		LastUpdated: nowStr,
		Kind:        "main",
	})
	lastUpdated := nowStr
	for _, turn := range turns {
		ts := turn.Timestamp.UTC()
		if ts.IsZero() {
			ts = now
		}
		lastUpdated = ts.Format(time.RFC3339Nano)
		lines = append(lines, turnToChatLine(turn, lastUpdated))
		lines = append(lines, map[string]any{
			"$set": map[string]any{"lastUpdated": lastUpdated},
		})
	}

	p := filepath.Join(chatDir, fmt.Sprintf(
		"session-%s-%s.jsonl",
		now.Format("2006-01-02T15-04"), sessionID[:8],
	))
	if err := writeJSONLines(p, lines); err != nil {
		return "", fmt.Errorf("gemini: write chat file: %w", err)
	}
	return sessionID, nil
}

// ResumeCmd returns the CLI command to resume an injected session.
// Delegates to kit's invocation facade so the argv stays in lockstep
// with the cross-CLI matrix in go/core/uxp/README.md.
func (a *Adapter) ResumeCmd(nativeID string) []string {
	return session.ResumeCmdFor(kitgemini.New(), nativeID)
}

func (a *Adapter) findChatFile(id string) (string, string, error) {
	tmpRoot := filepath.Join(a.geminiRoot(), "tmp")
	dirs, err := os.ReadDir(tmpRoot)
	if err != nil {
		return "", "", fmt.Errorf("gemini: read tmp dir: %w", err)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		chatDir := filepath.Join(tmpRoot, d.Name(), "chats")
		entries, err := os.ReadDir(chatDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			p := filepath.Join(chatDir, e.Name())
			meta, err := readChatHeader(p)
			if err == nil && sessionIDMatches(meta.SessionID, id) {
				return p, a.projectRootForAlias(d.Name()), nil
			}
		}
	}
	return "", "", fmt.Errorf("gemini: session %q not found", id)
}

func (a *Adapter) projectRootForAlias(alias string) string {
	p := filepath.Join(a.geminiRoot(), "tmp", alias, projectRootMarker)
	data, err := os.ReadFile(p)
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	pm, err := a.loadProjects()
	if err != nil {
		return ""
	}
	for cwd, mapped := range pm.Projects {
		if mapped == alias {
			return cwd
		}
	}
	return ""
}

func sessionIDMatches(sessionID, input string) bool {
	return sessionID == input || strings.HasPrefix(sessionID, input)
}

func readChatHeader(path string) (chatHeader, error) {
	lines, err := readJSONLines(path)
	if err != nil {
		return chatHeader{}, err
	}
	if len(lines) == 0 {
		return chatHeader{}, fmt.Errorf("empty chat file")
	}
	var h chatHeader
	if err := json.Unmarshal(lines[0], &h); err != nil {
		return chatHeader{}, err
	}
	return h, nil
}

func readJSONLines(path string) ([]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []json.RawMessage
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, json.RawMessage(line))
	}
	return lines, nil
}

func writeJSONLines(path string, lines []any) error {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, line := range lines {
		if err := enc.Encode(line); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func chatLineToTurn(line json.RawMessage) (session.Turn, bool) {
	var raw struct {
		Type      string             `json:"type"`
		Timestamp string             `json:"timestamp"`
		Content   json.RawMessage    `json:"content"`
		ToolCalls []session.ToolCall `json:"toolCalls"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return session.Turn{}, false
	}
	switch raw.Type {
	case "user", "gemini", "model", "assistant", "system":
	default:
		return session.Turn{}, false
	}
	ts, _ := time.Parse(time.RFC3339Nano, raw.Timestamp)
	return session.Turn{
		Role:      mapRole(raw.Type),
		Content:   parseChatContent(raw.Content),
		ToolCalls: raw.ToolCalls,
		Timestamp: ts,
	}, true
}

func parseChatContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []chatContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var out []string
		for _, p := range parts {
			out = append(out, p.Text)
		}
		return strings.Join(out, "\n")
	}
	return ""
}

func turnToChatLine(turn session.Turn, timestamp string) map[string]any {
	text := turn.Content
	for _, tc := range turn.ToolCalls {
		text += fmt.Sprintf("\n[Tool: %s → %s]", tc.Name, tc.Output)
	}
	switch turn.Role {
	case session.RoleAssistant:
		return map[string]any{
			"id":        mustGenerateUUID(),
			"timestamp": timestamp,
			"type":      "gemini",
			"content":   text,
			"thoughts":  []any{},
			"toolCalls": []any{},
		}
	case session.RoleSystem:
		text = "[System] " + text
	}
	return map[string]any{
		"id":        mustGenerateUUID(),
		"timestamp": timestamp,
		"type":      "user",
		"content":   []chatContentPart{{Text: text}},
	}
}

func projectHash(cwd string) string {
	sum := sha256.Sum256([]byte(cwd))
	return fmt.Sprintf("%x", sum[:])
}

func mustGenerateUUID() string {
	id, err := generateUUID()
	if err != nil {
		return fmt.Sprintf("usp-%d", time.Now().UnixNano())
	}
	return id
}

func generateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("gemini: generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	), nil
}

// geminiRoot returns the resolved ~/.gemini path.
func (a *Adapter) geminiRoot() string {
	home := a.HomeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".gemini")
}

// loadProjects reads and parses ~/.gemini/projects.json.
func (a *Adapter) loadProjects() (*projectsJSON, error) {
	data, err := os.ReadFile(filepath.Join(a.geminiRoot(), projectsFile))
	if err != nil {
		return nil, err
	}
	var pm projectsJSON
	if err := json.Unmarshal(data, &pm); err != nil {
		return nil, err
	}
	if pm.Projects == nil {
		pm.Projects = make(map[string]string)
	}
	return &pm, nil
}

// readModelFromTranscript best-effort pulls `assistant.model` from a
// Gemini chat JSONL file. Returns "" when no
// model field is present (Gemini schema is loose; future versions may
// add it). Errors are silent — telemetry is opportunistic.
func readModelFromTranscript(path string) string {
	lines, err := readJSONLines(path)
	if err != nil {
		return ""
	}
	var last string
	for _, line := range lines {
		var raw struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(line, &raw); err == nil && raw.Model != "" {
			last = raw.Model
		}
	}
	return last
}

// mapRole translates Gemini role strings to USP roles.
func mapRole(r string) session.Role {
	switch r {
	case "user":
		return session.RoleUser
	case "model", "assistant":
		return session.RoleAssistant
	case "system":
		return session.RoleSystem
	default:
		return session.RoleAssistant
	}
}

// capMap implements uxp.CapabilityMap for Gemini CLI.
type capMap struct{}

func (c *capMap) Supports(dim string) bool {
	s, ok := c.Coverage()[dim]
	return ok && s != uxp.Missing
}

func (c *capMap) Coverage() map[string]uxp.Support {
	return map[string]uxp.Support{
		// Native (10)
		"per-project-sessions":  uxp.Native,
		"session-alias":         uxp.Native,
		"session-resume":        uxp.Native,
		"resume-by-uuid":        uxp.Native,
		"named-save":            uxp.Native,
		"project-map":           uxp.Native,
		"worktree-isolation":    uxp.Native,
		"trusted-folders":       uxp.Native,
		"mcp-servers":           uxp.Native,
		"session-retention-cfg": uxp.Native,
		// Workaround (4)
		"transcript-read":     uxp.Workaround,
		"cross-project-list":  uxp.Workaround,
		"project-root-marker": uxp.Workaround,
		"auto-recent-resume":  uxp.Workaround,
		// Missing (6)
		"append-stream":         uxp.Missing,
		"memory-subsystem":      uxp.Missing,
		"cross-project-search":  uxp.Missing,
		"per-message-timestamp": uxp.Missing,
		"tool-call-capture":     uxp.Missing,
		"event-log":             uxp.Missing,
	}
}

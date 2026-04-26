// Package gemini implements session.SessionAdapter for Gemini CLI.
//
// Store layout:
//   ~/.gemini/projects.json   — cwd → alias map
//   ~/.gemini/history/<alias>/ — per-project history dir
//
// KNOWN LIMITATION (2026-04-11): history dirs contain only
// .project_root markers; no chat transcript JSON files have been
// observed on disk. The adapter handles this gracefully — ListSessions
// returns empty when no chat files exist, and GetSession returns a
// clear error when transcripts are absent.
package gemini

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

// projectsFile is the central cwd→alias map inside ~/.gemini/.
const projectsFile = "projects.json"

// projectRootMarker is the sentinel file inside each history dir.
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

// ListSessions returns sessions found under the history dir for cwd.
// Returns empty (not error) when the dir contains only .project_root
// markers — this is the expected real-world case as of 2026-04-11.
func (a *Adapter) ListSessions(cwd string) ([]session.Session, error) {
	histRoot := filepath.Join(a.geminiRoot(), "history")

	if cwd == "" {
		return a.listAllAliases(histRoot)
	}

	alias := a.ProjectKey(cwd)
	return a.listAlias(histRoot, alias, cwd)
}

func (a *Adapter) listAllAliases(histRoot string) ([]session.Session, error) {
	dirs, err := os.ReadDir(histRoot)
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
		sessions, _ := a.listAlias(histRoot, d.Name(), "")
		all = append(all, sessions...)
	}
	return all, nil
}

func (a *Adapter) listAlias(histRoot, alias, cwd string) ([]session.Session, error) {
	histDir := filepath.Join(histRoot, alias)
	entries, err := os.ReadDir(histDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("gemini: read history dir: %w", err)
	}

	var sessions []session.Session
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		tag := strings.TrimSuffix(name, ".json")
		info, _ := e.Info()
		s := session.Session{
			CLI:        uxp.CLIGemini,
			ProjectCwd: cwd,
			TurnCount:  0,
		}
		s.SetIDs(tag)
		if info != nil {
			s.StartedAt = info.ModTime()
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetSession returns a session by its tag ID. The tag corresponds to
// a <tag>.json file inside the project's history dir.
//
// Returns an error when the transcript file does not exist — this is
// the expected case as of 2026-04-11 since Gemini CLI does not appear
// to write chat files to disk without explicit /chat save.
func (a *Adapter) GetSession(id string) (*session.Session, error) {
	// We need to scan projects.json to find which cwd owns this tag.
	pm, err := a.loadProjects()
	if err != nil {
		return nil, fmt.Errorf("gemini: load projects: %w", err)
	}

	for cwd, alias := range pm.Projects {
		p := filepath.Join(a.geminiRoot(), "history", alias, id+".json")
		info, statErr := os.Stat(p)
		if statErr != nil {
			continue
		}
		s := &session.Session{
			CLI:        uxp.CLIGemini,
			ProjectCwd: cwd,
			StartedAt:  info.ModTime(),
		}
		s.SetIDs(id)
		return s, nil
	}

	return nil, fmt.Errorf(
		"gemini: session %q not found — transcript files may not "+
			"exist on disk (known limitation)", id)
}

// StreamTurns reads the chat JSON for the given session and emits one
// Turn per message entry. Returns an error if the file is absent.
func (a *Adapter) StreamTurns(id string) (<-chan session.Turn, error) {
	pm, err := a.loadProjects()
	if err != nil {
		return nil, fmt.Errorf("gemini: load projects: %w", err)
	}

	var chatPath string
	for _, alias := range pm.Projects {
		p := filepath.Join(a.geminiRoot(), "history", alias, id+".json")
		if _, statErr := os.Stat(p); statErr == nil {
			chatPath = p
			break
		}
	}
	if chatPath == "" {
		return nil, fmt.Errorf(
			"gemini: transcript %q not found on disk "+
				"(known limitation: chat files require explicit /chat save)", id)
	}

	data, err := os.ReadFile(chatPath)
	if err != nil {
		return nil, fmt.Errorf("gemini: read transcript: %w", err)
	}

	var raw struct {
		History []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			Parts   []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"history"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("gemini: parse transcript: %w", err)
	}

	ch := make(chan session.Turn, len(raw.History))
	for _, msg := range raw.History {
		content := msg.Content
		if content == "" && len(msg.Parts) > 0 {
			var texts []string
			for _, p := range msg.Parts {
				texts = append(texts, p.Text)
			}
			content = strings.Join(texts, "\n")
		}
		ch <- session.Turn{
			Role:      mapRole(msg.Role),
			Content:   content,
			Timestamp: time.Time{}, // Gemini transcripts lack per-message timestamps.
		}
	}
	close(ch)
	return ch, nil
}

// geminiPart mirrors the Gemini chat JSON part structure.
type geminiPart struct {
	Text string `json:"text"`
}

// geminiMsg mirrors a single Gemini chat history message.
type geminiMsg struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

// geminiChat is the top-level Gemini chat JSON structure.
type geminiChat struct {
	History []geminiMsg `json:"history"`
}

// InjectSession writes turns into Gemini's native history dir as a
// new chat JSON file, enabling cross-CLI resume via `gemini chat
// resume <tag>`.
//
// CAVEAT (2026-04-11): Gemini CLI has no observed chat files on disk.
// Whether it actually reads injected files on resume is unverified.
func (a *Adapter) InjectSession(cwd string, turns []session.Turn) (string, error) {
	alias := a.ProjectKey(cwd)
	histDir := filepath.Join(a.geminiRoot(), "history", alias)
	if err := os.MkdirAll(histDir, 0o755); err != nil {
		return "", fmt.Errorf("gemini: create history dir: %w", err)
	}

	var msgs []geminiMsg
	for _, t := range turns {
		role, text := "user", t.Content
		switch t.Role {
		case session.RoleAssistant:
			role = "model"
		case session.RoleSystem:
			text = "[System] " + text
		}
		// Append tool call summaries.
		for _, tc := range t.ToolCalls {
			text += fmt.Sprintf("\n[Tool: %s → %s]", tc.Name, tc.Output)
		}
		msgs = append(msgs, geminiMsg{
			Role:  role,
			Parts: []geminiPart{{Text: text}},
		})
	}

	tag, err := generateTag()
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(geminiChat{History: msgs}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("gemini: marshal chat: %w", err)
	}
	p := filepath.Join(histDir, tag+".json")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", fmt.Errorf("gemini: write chat file: %w", err)
	}
	return tag, nil
}

// ResumeCmd returns the CLI command to resume an injected session.
func (a *Adapter) ResumeCmd(nativeID string) []string {
	return []string{"gemini", "chat", "resume", nativeID}
}

// generateTag returns a "usp-resume-<8hex>" tag.
func generateTag() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gemini: generate tag: %w", err)
	}
	return "usp-resume-" + hex.EncodeToString(b), nil
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
		// Native (9)
		"per-project-sessions":  uxp.Native,
		"session-alias":         uxp.Native,
		"session-resume":        uxp.Native,
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
		// Missing (7)
		"append-stream":       uxp.Missing,
		"memory-subsystem":    uxp.Missing,
		"cross-project-search": uxp.Missing,
		"resume-by-uuid":      uxp.Missing,
		"per-message-timestamp": uxp.Missing,
		"tool-call-capture":   uxp.Missing,
		"event-log":           uxp.Missing,
	}
}

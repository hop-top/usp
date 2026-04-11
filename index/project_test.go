package index

import (
	"os"
	"path/filepath"
	"testing"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

func tempDB(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test-index.db")
}

func TestOpen_CreatesDB(t *testing.T) {
	path := tempDB(t)
	idx, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("DB file not created: %v", err)
	}
}

func TestRegisterAndLookup(t *testing.T) {
	idx, err := Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	cwd := "/home/user/project"
	entries := map[string]string{
		"claude":   "-home-user-project",
		"opencode": "abc123sha1hex",
	}

	if err := idx.Register(cwd, entries); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := idx.Lookup(cwd)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	for cli, want := range entries {
		if got[cli] != want {
			t.Errorf("Lookup[%s] = %q, want %q", cli, got[cli], want)
		}
	}
}

func TestLookup_UnknownCwd(t *testing.T) {
	idx, err := Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	got, err := idx.Lookup("/nonexistent")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestRegister_OverwritesExisting(t *testing.T) {
	idx, err := Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	cwd := "/home/user/project"

	// First write.
	if err := idx.Register(cwd, map[string]string{
		"claude": "old-key",
	}); err != nil {
		t.Fatalf("Register(1): %v", err)
	}

	// Overwrite.
	if err := idx.Register(cwd, map[string]string{
		"claude": "new-key",
	}); err != nil {
		t.Fatalf("Register(2): %v", err)
	}

	got, err := idx.Lookup(cwd)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got["claude"] != "new-key" {
		t.Errorf("expected overwritten key %q, got %q", "new-key", got["claude"])
	}
}

// mockAdapter is a minimal session.SessionAdapter for testing Scan.
type mockAdapter struct {
	cli uxp.CLIName
	key string
}

var _ session.SessionAdapter = (*mockAdapter)(nil)

func (m *mockAdapter) CLI() uxp.CLIName              { return m.cli }
func (m *mockAdapter) Detect() (*uxp.DetectResult, error) { return nil, nil }
func (m *mockAdapter) Capabilities() uxp.CapabilityMap    { return nil }
func (m *mockAdapter) ProjectKey(string) string            { return m.key }
func (m *mockAdapter) ListSessions(string) ([]session.Session, error) {
	return nil, nil
}
func (m *mockAdapter) GetSession(string) (*session.Session, error) {
	return nil, nil
}
func (m *mockAdapter) StreamTurns(string) (<-chan session.Turn, error) {
	return nil, nil
}

func TestScan_RefreshesIndex(t *testing.T) {
	idx, err := Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	cwd := "/home/user/myapp"

	// Seed the index with one CLI.
	if err := idx.Register(cwd, map[string]string{
		"claude": "old-claude-key",
	}); err != nil {
		t.Fatalf("Register seed: %v", err)
	}

	// Scan with two mock adapters.
	adapters := []session.SessionAdapter{
		&mockAdapter{cli: "claude", key: "refreshed-claude-key"},
		&mockAdapter{cli: "opencode", key: "oc-sha1-key"},
	}

	if err := idx.Scan(adapters); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	got, err := idx.Lookup(cwd)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if got["claude"] != "refreshed-claude-key" {
		t.Errorf("claude key = %q, want %q", got["claude"], "refreshed-claude-key")
	}
	if got["opencode"] != "oc-sha1-key" {
		t.Errorf("opencode key = %q, want %q", got["opencode"], "oc-sha1-key")
	}
}

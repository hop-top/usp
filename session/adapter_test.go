package session

import (
	"testing"

	"hop.top/kit/uxp"
)

// mockAdapter verifies the interface is satisfiable.
type mockAdapter struct{}

func (m *mockAdapter) CLI() uxp.CLIName                  { return uxp.CLIClaude }
func (m *mockAdapter) Detect() (*uxp.DetectResult, error) { return &uxp.DetectResult{Installed: true}, nil }
func (m *mockAdapter) Capabilities() uxp.CapabilityMap    { return nil }
func (m *mockAdapter) ListSessions(_ string) ([]Session, error) { return nil, nil }
func (m *mockAdapter) GetSession(_ string) (*Session, error)    { return nil, nil }
func (m *mockAdapter) StreamTurns(_ string) (<-chan Turn, error) { return nil, nil }
func (m *mockAdapter) ProjectKey(_ string) string               { return "" }

func TestMockSatisfiesSessionAdapter(t *testing.T) {
	var _ SessionAdapter = (*mockAdapter)(nil)
}

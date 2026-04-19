package sessionutil

import (
	"testing"
	"time"

	"hop.top/kit/uxp"
	"hop.top/usp/session"
)

type stubAdapter struct {
	sessions []session.Session
}

func (s *stubAdapter) CLI() uxp.CLIName        { return "stub" }
func (s *stubAdapter) ProjectKey(string) string { return "" }
func (s *stubAdapter) Detect() (*uxp.DetectResult, error) {
	return &uxp.DetectResult{Installed: true}, nil
}
func (s *stubAdapter) Capabilities() uxp.CapabilityMap {
	return nil
}
func (s *stubAdapter) ListSessions(string) ([]session.Session, error) {
	return s.sessions, nil
}
func (s *stubAdapter) GetSession(id string) (*session.Session, error) {
	for i, sess := range s.sessions {
		if sess.ID == id {
			return &s.sessions[i], nil
		}
	}
	return nil, nil
}
func (s *stubAdapter) StreamTurns(string) (<-chan session.Turn, error) {
	ch := make(chan session.Turn)
	close(ch)
	return ch, nil
}

func TestResolveSessionID_ExactMatch(t *testing.T) {
	a := &stubAdapter{sessions: []session.Session{
		{ID: "abc-123", CLI: "stub"},
	}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	sess, cli, _, err := ResolveSessionID("abc-123", adapters, []string{"stub"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID != "abc-123" {
		t.Errorf("ID = %q, want abc-123", sess.ID)
	}
	if cli != "stub" {
		t.Errorf("CLI = %q, want stub", cli)
	}
}

func TestResolveSessionID_PrefixMatch(t *testing.T) {
	a := &stubAdapter{sessions: []session.Session{
		{ID: "abc-123-unique", CLI: "stub"},
		{ID: "def-456-other", CLI: "stub"},
	}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	sess, _, _, err := ResolveSessionID("abc", adapters, []string{"stub"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID != "abc-123-unique" {
		t.Errorf("ID = %q, want abc-123-unique", sess.ID)
	}
}

func TestResolveSessionID_Ambiguous(t *testing.T) {
	a := &stubAdapter{sessions: []session.Session{
		{ID: "abc-111", CLI: "stub"},
		{ID: "abc-222", CLI: "stub"},
	}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	_, _, _, err := ResolveSessionID("abc", adapters, []string{"stub"})
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func TestResolveSessionID_NotFound(t *testing.T) {
	a := &stubAdapter{sessions: []session.Session{
		{ID: "abc-123", CLI: "stub"},
	}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	_, _, _, err := ResolveSessionID("zzz", adapters, []string{"stub"})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestResolveSessionID_SinceFilter(t *testing.T) {
	now := time.Now()
	a := &stubAdapter{sessions: []session.Session{
		{ID: "abc-old", CLI: "stub", StartedAt: now.Add(-48 * time.Hour)},
		{ID: "abc-new", CLI: "stub", StartedAt: now.Add(-1 * time.Hour)},
	}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	sess, _, _, err := ResolveSessionID("abc", adapters, []string{"stub"},
		ResolveOpts{Since: now.Add(-24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID != "abc-new" {
		t.Errorf("ID = %q, want abc-new", sess.ID)
	}
}

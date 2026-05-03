package sessionutil

import (
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

type stubAdapter struct {
	sessions []session.Session
}

func (s *stubAdapter) CLI() uxp.CLIName         { return "stub" }
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
func (s *stubAdapter) GetSession(native string) (*session.Session, error) {
	// Stub mirrors real adapters: native id is the on-disk handle.
	for i, sess := range s.sessions {
		if sess.NativeID == native {
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

// stubSession builds a Session whose ID/NativeID pair matches what
// real adapters produce — mandatory for the resolver tests.
func stubSession(native string) session.Session {
	s := session.Session{CLI: "stub"}
	s.SetIDs(native)
	return s
}

func TestResolveSessionID_NativeExactMatch(t *testing.T) {
	s1 := stubSession("abc-123")
	a := &stubAdapter{sessions: []session.Session{s1}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	sess, cli, _, err := ResolveSessionID("abc-123", adapters, []string{"stub"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.NativeID != "abc-123" {
		t.Errorf("NativeID = %q, want abc-123", sess.NativeID)
	}
	if cli != "stub" {
		t.Errorf("CLI = %q, want stub", cli)
	}
}

func TestResolveSessionID_TypeIDExactMatch(t *testing.T) {
	s1 := stubSession("12341234-1234-4234-9234-123412341234")
	a := &stubAdapter{sessions: []session.Session{s1}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	sess, _, _, err := ResolveSessionID(s1.ID, adapters, []string{"stub"})
	if err != nil {
		t.Fatalf("resolve typeid: %v", err)
	}
	if sess.NativeID != s1.NativeID {
		t.Errorf("NativeID = %q, want %q", sess.NativeID, s1.NativeID)
	}
}

func TestResolveSessionID_NativePrefixMatch(t *testing.T) {
	s1 := stubSession("abc-123-unique")
	s2 := stubSession("def-456-other")
	a := &stubAdapter{sessions: []session.Session{s1, s2}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	sess, _, _, err := ResolveSessionID("abc", adapters, []string{"stub"})
	if err != nil {
		t.Fatal(err)
	}
	if sess.NativeID != "abc-123-unique" {
		t.Errorf("NativeID = %q, want abc-123-unique", sess.NativeID)
	}
}

func TestResolveSessionID_TypeIDPrefixMatch(t *testing.T) {
	s1 := stubSession("11111111-2222-4333-8444-555555555555")
	s2 := stubSession("aaaaaaaa-2222-4333-8444-555555555555")
	a := &stubAdapter{sessions: []session.Session{s1, s2}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	// First 8 chars of TypeID (sess_) is unique enough to disambiguate.
	prefix := s1.ID[:10]
	sess, _, _, err := ResolveSessionID(prefix, adapters, []string{"stub"})
	if err != nil {
		t.Fatalf("resolve typeid prefix: %v", err)
	}
	if sess.ID != s1.ID {
		t.Errorf("ID = %q, want %q", sess.ID, s1.ID)
	}
}

func TestResolveSessionID_Ambiguous(t *testing.T) {
	s1 := stubSession("abc-111")
	s2 := stubSession("abc-222")
	a := &stubAdapter{sessions: []session.Session{s1, s2}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	_, _, _, err := ResolveSessionID("abc", adapters, []string{"stub"})
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func TestResolveSessionID_NotFound(t *testing.T) {
	s1 := stubSession("abc-123")
	a := &stubAdapter{sessions: []session.Session{s1}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	_, _, _, err := ResolveSessionID("zzz", adapters, []string{"stub"})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestResolveSessionID_SinceFilter(t *testing.T) {
	now := time.Now()
	old := stubSession("abc-old")
	old.StartedAt = now.Add(-48 * time.Hour)
	fresh := stubSession("abc-new")
	fresh.StartedAt = now.Add(-1 * time.Hour)
	a := &stubAdapter{sessions: []session.Session{old, fresh}}
	adapters := map[string]session.SessionAdapter{"stub": a}
	sess, _, _, err := ResolveSessionID("abc", adapters, []string{"stub"},
		ResolveOpts{Since: now.Add(-24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if sess.NativeID != "abc-new" {
		t.Errorf("NativeID = %q, want abc-new", sess.NativeID)
	}
}

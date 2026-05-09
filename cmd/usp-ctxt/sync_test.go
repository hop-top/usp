package main

import (
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

func TestAdapterSourceTurnsUsesNativeIDFromListedSession(t *testing.T) {
	fake := &fakeSessionAdapter{
		sessions: []session.Session{{
			ID:        "sess_canonical",
			NativeID:  "native-session-id",
			CLI:       uxp.CLICodex,
			StartedAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
		}},
		turns: []session.Turn{{
			Role:    session.RoleUser,
			Content: "hello",
		}},
	}
	src := &adapterSource{
		clis:       []string{uxp.CLICodex},
		adapters:   map[string]session.SessionAdapter{uxp.CLICodex: fake},
		nativeByID: map[string]string{},
	}

	listed, err := src.ListSince(uxp.CLICodex, time.Time{})
	if err != nil {
		t.Fatalf("ListSince: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("ListSince len = %d, want 1", len(listed))
	}

	turns, err := src.Turns(uxp.CLICodex, listed[0].ID)
	if err != nil {
		t.Fatalf("Turns: %v", err)
	}
	if fake.streamID != "native-session-id" {
		t.Fatalf("StreamTurns id = %q, want native-session-id", fake.streamID)
	}
	if len(turns) != 1 || turns[0].Content != "hello" {
		t.Fatalf("Turns = %+v, want hello turn", turns)
	}
}

type fakeSessionAdapter struct {
	sessions []session.Session
	turns    []session.Turn
	streamID string
}

func (f *fakeSessionAdapter) CLI() uxp.CLIName { return uxp.CLICodex }

func (f *fakeSessionAdapter) Detect() (*uxp.DetectResult, error) {
	return &uxp.DetectResult{Installed: true}, nil
}

func (f *fakeSessionAdapter) Capabilities() uxp.CapabilityMap {
	return fakeCapabilities{}
}

func (f *fakeSessionAdapter) ListSessions(string) ([]session.Session, error) {
	return f.sessions, nil
}

func (f *fakeSessionAdapter) GetSession(id string) (*session.Session, error) {
	for _, sess := range f.sessions {
		if sess.ID == id || sess.NativeID == id {
			return &sess, nil
		}
	}
	return nil, nil
}

func (f *fakeSessionAdapter) StreamTurns(id string) (<-chan session.Turn, error) {
	f.streamID = id
	ch := make(chan session.Turn, len(f.turns))
	for _, turn := range f.turns {
		ch <- turn
	}
	close(ch)
	return ch, nil
}

func (f *fakeSessionAdapter) ProjectKey(cwd string) string { return cwd }

type fakeCapabilities struct{}

func (fakeCapabilities) Supports(string) bool { return false }

func (fakeCapabilities) Coverage() map[string]uxp.Support {
	return map[string]uxp.Support{}
}

package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

func TestListSessionsFiltersSortsAndLimits(t *testing.T) {
	t1 := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	svc := New(map[string]session.SessionAdapter{
		uxp.CLICodex: &fakeAdapter{sessions: []session.Session{
			mkSession("old", "native-old", uxp.CLICodex, t1),
			mkSession("new", "native-new", uxp.CLICodex, t2),
		}},
	})

	got, err := svc.ListSessions(context.Background(), ListSessionsRequest{
		CLI:   uxp.CLICodex,
		Since: t1.Add(30 * time.Minute),
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("ListSessions = %+v, want only newest session", got)
	}
}

func TestListSessionsUsesKitCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-cache.db")
	cache, err := OpenCache(path, time.Hour)
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	defer cache.Close()

	a := &fakeAdapter{sessions: []session.Session{
		mkSession("cached", "native-cached", uxp.CLICodex, time.Now()),
	}}
	svc := New(map[string]session.SessionAdapter{
		uxp.CLICodex: a,
	}, WithCache(cache))

	first, err := svc.ListSessions(context.Background(), ListSessionsRequest{
		CLI: uxp.CLICodex,
	})
	if err != nil {
		t.Fatalf("ListSessions first: %v", err)
	}
	a.sessions = []session.Session{
		mkSession("fresh", "native-fresh", uxp.CLICodex, time.Now()),
	}
	second, err := svc.ListSessions(context.Background(), ListSessionsRequest{
		CLI: uxp.CLICodex,
	})
	if err != nil {
		t.Fatalf("ListSessions second: %v", err)
	}
	if len(first) != 1 || len(second) != 1 || second[0].ID != first[0].ID {
		t.Fatalf("cached list = %+v, want %+v", second, first)
	}
}

func TestSearchSessionsStreamsNativeID(t *testing.T) {
	a := &fakeAdapter{
		sessions: []session.Session{
			mkSession("sess_canonical", "native-session", uxp.CLICodex, time.Now()),
		},
		turns: []session.Turn{{Role: session.RoleUser, Content: "needle"}},
	}
	svc := New(map[string]session.SessionAdapter{uxp.CLICodex: a})

	got, err := svc.SearchSessions(context.Background(), SearchSessionsRequest{
		CLI:   uxp.CLICodex,
		Query: "needle",
	})
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SearchSessions len = %d, want 1", len(got))
	}
	if a.streamID != "native-session" {
		t.Fatalf("StreamTurns id = %q, want native-session", a.streamID)
	}
}

func TestListSessionItemsSummarizesTurns(t *testing.T) {
	a := &fakeAdapter{
		sessions: []session.Session{
			mkSession("sess_canonical", "native-session", uxp.CLICodex, time.Now()),
		},
		turns: []session.Turn{
			{Role: session.RoleUser, Content: "make list useful"},
			{Role: session.RoleAssistant, Content: "Implemented better list columns. Extra detail follows."},
		},
	}
	svc := New(map[string]session.SessionAdapter{uxp.CLICodex: a})

	got, err := svc.ListSessionItems(context.Background(), ListSessionsRequest{
		CLI: uxp.CLICodex,
	})
	if err != nil {
		t.Fatalf("ListSessionItems: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListSessionItems len = %d, want 1", len(got))
	}
	if got[0].Actions != "Implemented better list columns." {
		t.Fatalf("Actions = %q", got[0].Actions)
	}
	if a.streamID != "native-session" {
		t.Fatalf("StreamTurns id = %q, want native-session", a.streamID)
	}
}

func TestShowSessionReturnsDetail(t *testing.T) {
	started := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	a := &fakeAdapter{
		sessions: []session.Session{
			mkSession("sess_canonical", "native-session", uxp.CLICodex, started),
		},
		turns: []session.Turn{{Role: session.RoleAssistant, Content: "answer"}},
	}
	svc := New(map[string]session.SessionAdapter{uxp.CLICodex: a})

	got, err := svc.ShowSession(context.Background(), ShowSessionRequest{
		ID:            "sess_canonical",
		CLI:           uxp.CLICodex,
		IncludeSkills: true,
	})
	if err != nil {
		t.Fatalf("ShowSession: %v", err)
	}
	if got.Session.NativeID != "native-session" || got.CLI != uxp.CLICodex {
		t.Fatalf("ShowSession detail = %+v, want codex native-session", got)
	}
	if len(got.Turns) != 1 || got.Turns[0].Content != "answer" {
		t.Fatalf("turns = %+v, want answer", got.Turns)
	}
	if len(got.Skills) != 1 || !got.Skills[0].Unsupported {
		t.Fatalf("skills = %+v, want unsupported marker", got.Skills)
	}
}

func TestListSkillEventsUnsupportedAdapter(t *testing.T) {
	started := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	svc := New(map[string]session.SessionAdapter{
		uxp.CLICodex: &fakeAdapter{sessions: []session.Session{
			mkSession("sess_canonical", "native-session", uxp.CLICodex, started),
		}},
	})

	got, err := svc.ListSkillEvents(context.Background(), ListSkillEventsRequest{
		CLI:   uxp.CLICodex,
		Since: started.Add(-time.Minute),
		Until: started.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ListSkillEvents: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != "native-session" || !got[0].Unsupported {
		t.Fatalf("ListSkillEvents = %+v, want unsupported native-session", got)
	}
}

func TestResumeSessionInjectsAndRecordsLineage(t *testing.T) {
	source := &fakeAdapter{
		sessions: []session.Session{
			mkSession("sess_source", "native-source", uxp.CLICodex, time.Now()),
		},
		turns: []session.Turn{{Role: session.RoleUser, Content: "continue"}},
	}
	target := &fakeResumeAdapter{nativeID: "native-target"}
	svc := New(map[string]session.SessionAdapter{
		uxp.CLICodex:  source,
		uxp.CLIClaude: target,
	})

	got, err := svc.ResumeSession(context.Background(), ResumeSessionRequest{
		ID:          "sess_source",
		TargetCLI:   uxp.CLIClaude,
		ProjectCWD:  "/tmp/project",
		LineagePath: t.TempDir() + "/sessions.db",
	})
	if err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if source.streamID != "native-source" {
		t.Fatalf("source StreamTurns id = %q, want native-source", source.streamID)
	}
	if len(target.injectedTurns) != 1 || target.injectedTurns[0].Content != "continue" {
		t.Fatalf("injectedTurns = %+v, want continue", target.injectedTurns)
	}
	if got.TargetNative != "native-target" || got.Command[0] != "claude" {
		t.Fatalf("ResumeSession result = %+v, want claude native-target", got)
	}
}

type fakeResumeAdapter struct {
	fakeAdapter
	nativeID      string
	injectedTurns []session.Turn
}

func (f *fakeResumeAdapter) CLI() uxp.CLIName { return uxp.CLIClaude }

func (f *fakeResumeAdapter) InjectSession(_ string, turns []session.Turn) (string, error) {
	f.injectedTurns = append([]session.Turn(nil), turns...)
	return f.nativeID, nil
}

func (f *fakeResumeAdapter) ResumeCmd(nativeID string) []string {
	return []string{"claude", "--resume", nativeID}
}

func mkSession(id, native string, cli uxp.CLIName, started time.Time) session.Session {
	return session.Session{
		ID:        id,
		NativeID:  native,
		CLI:       cli,
		StartedAt: started,
		TurnCount: 1,
	}
}

type fakeAdapter struct {
	sessions []session.Session
	turns    []session.Turn
	streamID string
}

func (f *fakeAdapter) CLI() uxp.CLIName { return uxp.CLICodex }

func (f *fakeAdapter) Detect() (*uxp.DetectResult, error) {
	return &uxp.DetectResult{Installed: true}, nil
}

func (f *fakeAdapter) Capabilities() uxp.CapabilityMap { return fakeCaps{} }

func (f *fakeAdapter) ListSessions(string) ([]session.Session, error) {
	return f.sessions, nil
}

func (f *fakeAdapter) GetSession(id string) (*session.Session, error) {
	for _, sess := range f.sessions {
		if sess.ID == id || sess.NativeID == id {
			return &sess, nil
		}
	}
	return nil, nil
}

func (f *fakeAdapter) StreamTurns(id string) (<-chan session.Turn, error) {
	f.streamID = id
	ch := make(chan session.Turn, len(f.turns))
	for _, turn := range f.turns {
		ch <- turn
	}
	close(ch)
	return ch, nil
}

func (f *fakeAdapter) ProjectKey(cwd string) string { return cwd }

type fakeCaps struct{}

func (fakeCaps) Supports(string) bool { return false }

func (fakeCaps) Coverage() map[string]uxp.Support {
	return map[string]uxp.Support{}
}

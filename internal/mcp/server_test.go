package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/internal/api"
	"hop.top/usp/session"
)

func TestHandleInitialize(t *testing.T) {
	srv := New(api.New(nil))
	resp, ok := srv.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	if !ok {
		t.Fatal("expected response")
	}
	got := marshalMap(t, resp)
	if got["error"] != nil {
		t.Fatalf("unexpected error: %+v", got["error"])
	}
	result := got["result"].(map[string]any)
	if result["protocolVersion"] == "" {
		t.Fatalf("missing protocolVersion in %+v", result)
	}
}

func TestHandleToolsList(t *testing.T) {
	srv := New(api.New(nil))
	resp, ok := srv.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":"x","method":"tools/list"}`))
	if !ok {
		t.Fatal("expected response")
	}
	got := marshalMap(t, resp)
	result := got["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 4 {
		t.Fatalf("tools len = %d, want 4", len(tools))
	}
	first := tools[0].(map[string]any)
	if first["name"] != "usp_session_list" {
		t.Fatalf("first tool = %+v", first)
	}
}

func TestHandleToolCallSessionList(t *testing.T) {
	started := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)
	svc := api.New(map[string]session.SessionAdapter{
		uxp.CLICodex: &fakeAdapter{sessions: []session.Session{{
			ID:        "sess_123",
			NativeID:  "native-123",
			CLI:       uxp.CLICodex,
			StartedAt: started,
			TurnCount: 2,
		}}},
	})
	srv := New(svc)

	resp, ok := srv.Handle(context.Background(), []byte(`{
		"jsonrpc":"2.0",
		"id":2,
		"method":"tools/call",
		"params":{"name":"usp_session_list","arguments":{"tool":"codex","limit":1}}
	}`))
	if !ok {
		t.Fatal("expected response")
	}
	got := marshalMap(t, resp)
	result := got["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	if !strings.Contains(content["text"].(string), "sess_123") {
		t.Fatalf("tool text = %s, want session id", content["text"])
	}
}

func marshalMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

type fakeAdapter struct {
	sessions []session.Session
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

func (f *fakeAdapter) StreamTurns(string) (<-chan session.Turn, error) {
	ch := make(chan session.Turn)
	close(ch)
	return ch, nil
}

func (f *fakeAdapter) ProjectKey(cwd string) string { return cwd }

type fakeCaps struct{}

func (fakeCaps) Supports(string) bool { return false }

func (fakeCaps) Coverage() map[string]uxp.Support {
	return map[string]uxp.Support{}
}

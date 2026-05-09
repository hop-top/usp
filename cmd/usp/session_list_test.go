package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"hop.top/kit/go/console/output"
	"hop.top/kit/go/core/uxp"
	"hop.top/usp/internal/api"
	"hop.top/usp/session"
)

func TestSessionListCmd_Flags(t *testing.T) {
	cmd := sessionListCmd()

	if cmd.Use != "list" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "list")
	}

	// --format is inherited from root persistent flag.
	for _, name := range []string{"project", "tool", "since", "limit"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag %q", name)
		}
	}

	lim, err := cmd.Flags().GetInt("limit")
	if err != nil {
		t.Fatal(err)
	}
	if lim != 20 {
		t.Errorf("limit default = %d, want 20", lim)
	}

	projectFlag := cmd.Flags().Lookup("project")
	if projectFlag.DefValue == "" {
		t.Error("project default should be current working directory")
	}
	if !strings.Contains(projectFlag.Usage, "falls back to all projects") {
		t.Errorf("project help = %q, want fallback wording", projectFlag.Usage)
	}
}

func TestSessionListCmd_Wired(t *testing.T) {
	parent := sessionCmd(nil)
	found := false
	for _, sub := range parent.Commands() {
		if sub.Use == "list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("session list not wired into session parent command")
	}
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		in  string
		max int
		out string
	}{
		{"short", 12, "short"},
		{"abcdefghijklmnop", 12, "abcdefghijkl…"},
	}
	for _, tt := range tests {
		got := truncateID(tt.in, tt.max)
		if got != tt.out {
			t.Errorf("truncateID(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.out)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
		{now.Add(-48 * time.Hour), "2d ago"},
		{now.Add(-24 * time.Hour), "1d ago"},
	}
	for _, tt := range tests {
		got := relativeTime(tt.t)
		if got != tt.want {
			t.Errorf("relativeTime(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestToItemRowsDefaultColumns(t *testing.T) {
	items := []api.SessionListItem{{
		Session: session.Session{
			ID:         "sess_abcdefghijklmnopqrstuvwxyz",
			NativeID:   "native",
			CLI:        uxp.CLICodex,
			ProjectCwd: "/tmp/work/usp",
			StartedAt:  time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
			TurnCount:  3,
		},
		Actions: "implemented list projection",
	}}
	rows := toItemRows(items, output.Table)
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1", len(rows))
	}
	if rows[0].ID == "" || strings.Contains(rows[0].ID, "…") {
		t.Fatalf("ID = %q, want queryable prefix without ellipsis", rows[0].ID)
	}
	if rows[0].Source != "codex" {
		t.Fatalf("Source = %q, want codex for non-TTY", rows[0].Source)
	}
	if rows[0].Project != "usp" {
		t.Fatalf("Project = %q, want basename", rows[0].Project)
	}
	if rows[0].Actions != "implemented list projection" {
		t.Fatalf("Actions = %q", rows[0].Actions)
	}
}

func TestQueryableIDExtendsUntilUnique(t *testing.T) {
	all := []string{"sess_abcdef111", "sess_abcdef222"}
	got := queryableID("sess_abcdef111", all)
	if got != "sess_abcdef1" {
		t.Fatalf("queryableID = %q, want unique prefix", got)
	}
}

func TestListSessionsWithProjectFallback(t *testing.T) {
	svc := &fakeSessionLister{
		responses: map[string][]session.Session{
			"": {
				{ID: "sess_all", NativeID: "native-all", CLI: uxp.CLICodex},
			},
		},
	}
	got, err := listSessionsWithProjectFallback(
		context.Background(),
		svc,
		api.ListSessionsRequest{Project: "/tmp/project"},
		true,
	)
	if err != nil {
		t.Fatalf("listSessionsWithProjectFallback: %v", err)
	}
	if len(got) != 1 || got[0].ID != "sess_all" {
		t.Fatalf("fallback sessions = %+v, want all-project session", got)
	}
	if len(svc.calls) != 2 || svc.calls[0] != "/tmp/project" || svc.calls[1] != "" {
		t.Fatalf("calls = %+v, want project then all projects", svc.calls)
	}
}

func TestListSessionsWithProjectFallbackKeepsExplicitProject(t *testing.T) {
	svc := &fakeSessionLister{}
	got, err := listSessionsWithProjectFallback(
		context.Background(),
		svc,
		api.ListSessionsRequest{Project: "/tmp/project"},
		false,
	)
	if err != nil {
		t.Fatalf("listSessionsWithProjectFallback: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("sessions = %+v, want none", got)
	}
	if len(svc.calls) != 1 || svc.calls[0] != "/tmp/project" {
		t.Fatalf("calls = %+v, want explicit project only", svc.calls)
	}
}

func TestListSessionItemsWithProjectFallback(t *testing.T) {
	svc := &fakeSessionItemLister{
		responses: map[string][]api.SessionListItem{
			"": {
				{Session: session.Session{ID: "sess_all", NativeID: "native-all", CLI: uxp.CLICodex}},
			},
		},
	}
	got, err := listSessionItemsWithProjectFallback(
		context.Background(),
		svc,
		api.ListSessionsRequest{Project: "/tmp/project"},
		true,
	)
	if err != nil {
		t.Fatalf("listSessionItemsWithProjectFallback: %v", err)
	}
	if len(got) != 1 || got[0].Session.ID != "sess_all" {
		t.Fatalf("fallback items = %+v, want all-project item", got)
	}
	if len(svc.calls) != 2 || svc.calls[0] != "/tmp/project" || svc.calls[1] != "" {
		t.Fatalf("calls = %+v, want project then all projects", svc.calls)
	}
}

type fakeSessionLister struct {
	responses map[string][]session.Session
	calls     []string
}

type fakeSessionItemLister struct {
	responses map[string][]api.SessionListItem
	calls     []string
}

func (f *fakeSessionItemLister) ListSessionItems(
	_ context.Context,
	req api.ListSessionsRequest,
) ([]api.SessionListItem, error) {
	f.calls = append(f.calls, req.Project)
	return f.responses[req.Project], nil
}

func (f *fakeSessionLister) ListSessions(
	_ context.Context,
	req api.ListSessionsRequest,
) ([]session.Session, error) {
	f.calls = append(f.calls, req.Project)
	return f.responses[req.Project], nil
}

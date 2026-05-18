package uspctxt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/session"
)

// fakeSource is a deterministic SessionSource for unit tests.
type fakeSource struct {
	clis     []string
	lists    map[string][]session.Session
	turns    map[string][]session.Turn
	roots    map[string]string
	listErr  error
	turnsErr error
}

func (f *fakeSource) CLIs() []string { return f.clis }

func (f *fakeSource) ListSince(cli string, since time.Time) ([]session.Session, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	all := f.lists[cli]
	if since.IsZero() {
		return all, nil
	}
	out := []session.Session{}
	for _, s := range all {
		if s.StartedAt.After(since) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSource) Turns(_ , id string) ([]session.Turn, error) {
	if f.turnsErr != nil {
		return nil, f.turnsErr
	}
	return f.turns[id], nil
}

func (f *fakeSource) LineageRoot(id string) (string, error) {
	return f.roots[id], nil
}

// recordingClient captures projections for assertions.
type recordingClient struct {
	calls []Projection
	fail  map[string]error
}

func (r *recordingClient) Upsert(_ context.Context, p Projection) error {
	if err := r.fail[p.SourceKey]; err != nil {
		return err
	}
	r.calls = append(r.calls, p)
	return nil
}

func mkSession(id, cli string, started time.Time) session.Session {
	return session.Session{
		ID:         id,
		CLI:        uxp.CLIName(cli),
		ProjectCwd: "/tmp/p",
		StartedAt:  started,
		TurnCount:  1,
	}
}

func TestRun_IngestsAndAdvancesHWM(t *testing.T) {
	t1 := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)
	src := &fakeSource{
		clis: []string{"claude"},
		lists: map[string][]session.Session{
			"claude": {mkSession("s-1", "claude", t1), mkSession("s-2", "claude", t2)},
		},
		turns: map[string][]session.Turn{
			"s-1": {{Role: session.RoleUser, Content: "hi"}},
			"s-2": {{Role: session.RoleUser, Content: "hello"}},
		},
	}
	rc := &recordingClient{}
	st := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}

	res, err := Run(context.Background(), src, rc, st, RunOpts{Agent: "sami"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Ingested["claude"]; got != 2 {
		t.Errorf("ingested claude: want 2, got %d", got)
	}
	if !st.HWM("claude").Equal(t2) {
		t.Errorf("hwm should be t2=%v; got %v", t2, st.HWM("claude"))
	}
	if len(rc.calls) != 2 {
		t.Errorf("client calls: want 2, got %d", len(rc.calls))
	}
}

func TestRun_Idempotent_ReRunSkipsAll(t *testing.T) {
	t1 := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	src := &fakeSource{
		clis:  []string{"claude"},
		lists: map[string][]session.Session{"claude": {mkSession("s-1", "claude", t1)}},
		turns: map[string][]session.Turn{"s-1": {{Role: session.RoleUser, Content: "hi"}}},
	}
	rc := &recordingClient{}
	st := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}

	if _, err := Run(context.Background(), src, rc, st, RunOpts{}); err != nil {
		t.Fatalf("Run #1: %v", err)
	}
	rc.calls = nil
	if _, err := Run(context.Background(), src, rc, st, RunOpts{}); err != nil {
		t.Fatalf("Run #2: %v", err)
	}
	if len(rc.calls) != 0 {
		t.Errorf("re-run should not call ctxt; got %d calls", len(rc.calls))
	}
}

func TestRun_PartialFailure_HWMOnlyAdvancesPastSuccesses(t *testing.T) {
	t1 := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	src := &fakeSource{
		clis: []string{"claude"},
		lists: map[string][]session.Session{
			"claude": {
				mkSession("s-1", "claude", t1),
				mkSession("s-2", "claude", t2),
				mkSession("s-3", "claude", t3),
			},
		},
		turns: map[string][]session.Turn{
			"s-1": {{Role: session.RoleUser, Content: "hi"}},
			"s-2": {{Role: session.RoleUser, Content: "hi"}},
			"s-3": {{Role: session.RoleUser, Content: "hi"}},
		},
	}
	rc := &recordingClient{
		fail: map[string]error{"usp/s-2": errors.New("boom")},
	}
	st := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}

	res, err := Run(context.Background(), src, rc, st, RunOpts{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Failed["claude"]; got != 1 {
		t.Errorf("failed: want 1, got %d", got)
	}
	if got := res.Ingested["claude"]; got != 2 {
		t.Errorf("ingested: want 2, got %d", got)
	}
	// hwm should sit at t1 (last contiguous success before failure);
	// or at t3 if we accept "advance past any success". Spec §4.5 says
	// hwm advances only past successes — strictly increasing as we
	// iterate; t3 wins since it succeeded after t2 failed.
	if !st.HWM("claude").Equal(t3) {
		t.Errorf("hwm: want t3=%v (last success); got %v", t3, st.HWM("claude"))
	}
}

func TestRun_FilterByHWM(t *testing.T) {
	t1 := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 25, 11, 0, 0, 0, time.UTC)
	src := &fakeSource{
		clis: []string{"claude"},
		lists: map[string][]session.Session{
			"claude": {mkSession("s-old", "claude", t1), mkSession("s-new", "claude", t2)},
		},
		turns: map[string][]session.Turn{
			"s-old": {{Role: session.RoleUser, Content: "old"}},
			"s-new": {{Role: session.RoleUser, Content: "new"}},
		},
	}
	rc := &recordingClient{}
	st := &State{PerCLI: map[string]CLIState{"claude": {LastStartedAt: t1}}, Version: SchemaVersion}

	res, _ := Run(context.Background(), src, rc, st, RunOpts{})
	if got := res.Ingested["claude"]; got != 1 {
		t.Errorf("only s-new should ingest; got %d", got)
	}
	if len(rc.calls) != 1 || rc.calls[0].SourceKey != "usp/s-new" {
		t.Errorf("expected single call for s-new; got %+v", rc.calls)
	}
}

func TestExecClient_Upsert_HappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake ctxt is POSIX sh")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	bin := filepath.Join(dir, "ctxt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + logPath + " ; cat >> " + logPath + " ; exit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ctxt: %v", err)
	}
	prev := CtxtBin
	CtxtBin = bin
	t.Cleanup(func() { CtxtBin = prev })

	c := NewExecClient("")
	p := Project(sampleSession(), sampleTurns(), ProjectOpts{Agent: "sami"})
	if err := c.Upsert(context.Background(), p); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	s := string(got)
	// Primary identity = mentions; --source-key retained as secondary
	// dedup hint per spec §4.4. --hints emitted only if non-empty.
	for _, w := range []string{
		"analyze",
		"--mentions",
		"@usp.session.fe2eb947-ecab-4293-a26c-3485062e8e6a",
		"@agent.sami",
		"@cli.claude",
		"--source-key",
		p.SourceKey,
		"--wait",
	} {
		if !contains(s, w) {
			t.Errorf("expected fake ctxt to receive %q; got log:\n%s", w, s)
		}
	}
}

// TestExecClient_OmitsEmptyHints: post-mentions, hints may be empty when
// content-hash is the only signal — flag must not be passed empty.
func TestExecClient_OmitsEmptyHints(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake ctxt is POSIX sh")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	bin := filepath.Join(dir, "ctxt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + logPath + " ; cat >> " + logPath + " ; exit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake ctxt: %v", err)
	}
	prev := CtxtBin
	CtxtBin = bin
	t.Cleanup(func() { CtxtBin = prev })

	c := NewExecClient("")
	// Hand-built projection with empty Hints.
	p := Projection{
		SourceKey:   "usp/empty-hints",
		Mentions:    []string{"@usp.session.empty-hints", "@cli.claude"},
		Hints:       nil,
		Body:        "stub",
		ContentHash: "deadbeef",
	}
	if err := c.Upsert(context.Background(), p); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, _ := os.ReadFile(logPath)
	s := string(got)
	if contains(s, "--hints") {
		t.Errorf("--hints should be omitted when Hints empty; got log:\n%s", s)
	}
	if !contains(s, "--mentions") {
		t.Errorf("--mentions must always be passed; got log:\n%s", s)
	}
}

// TestRun_Idempotent_MentionStableAcrossRuns: re-projecting the same session
// produces identical Mentions list on a fresh Run, even when state is reset.
// Belt-and-suspenders for spec §4.4 mention-based identity.
func TestRun_Idempotent_MentionStableAcrossRuns(t *testing.T) {
	t1 := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	src := &fakeSource{
		clis:  []string{"claude"},
		lists: map[string][]session.Session{"claude": {mkSession("s-stable", "claude", t1)}},
		turns: map[string][]session.Turn{"s-stable": {{Role: session.RoleUser, Content: "hi"}}},
	}
	rc1 := &recordingClient{}
	rc2 := &recordingClient{}
	st1 := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}
	st2 := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}

	if _, err := Run(context.Background(), src, rc1, st1, RunOpts{Agent: "sami"}); err != nil {
		t.Fatalf("Run #1: %v", err)
	}
	if _, err := Run(context.Background(), src, rc2, st2, RunOpts{Agent: "sami"}); err != nil {
		t.Fatalf("Run #2: %v", err)
	}
	if len(rc1.calls) != 1 || len(rc2.calls) != 1 {
		t.Fatalf("expected 1 call per run; got %d / %d", len(rc1.calls), len(rc2.calls))
	}
	a, b := rc1.calls[0].MentionsString(), rc2.calls[0].MentionsString()
	if a != b {
		t.Errorf("mentions not stable across runs:\n  a=%q\n  b=%q", a, b)
	}
	want := "@usp.session.s-stable"
	if !strings.Contains(a, want) {
		t.Errorf("expected mention %q in %q", want, a)
	}
}

func TestExecClient_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake ctxt is POSIX sh")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "ctxt")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatalf("write fake ctxt: %v", err)
	}
	prev := CtxtBin
	CtxtBin = bin
	t.Cleanup(func() { CtxtBin = prev })

	c := NewExecClient("")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := c.Upsert(ctx, Projection{Body: "x", Hints: []string{"#x"}, SourceKey: "usp/x"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// contains is a tiny shim used by the bridge_test exec assertions.
func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Integration smoke against a real ctxt server.
// Gated on USP_CTXT_INTEGRATION=1.
func TestRun_Integration_RealCtxt(t *testing.T) {
	if os.Getenv("USP_CTXT_INTEGRATION") != "1" {
		t.Skip("set USP_CTXT_INTEGRATION=1 to run; needs local dpkms + ctxt on PATH")
	}
	t1 := time.Now().Add(-time.Hour)
	src := &fakeSource{
		clis: []string{"claude"},
		lists: map[string][]session.Session{
			"claude": {mkSession("integration-smoke-t0057", "claude", t1)},
		},
		turns: map[string][]session.Turn{
			"integration-smoke-t0057": {{Role: session.RoleUser, Content: "hello from T-0057 test"}},
		},
	}
	st := &State{PerCLI: map[string]CLIState{}, Version: SchemaVersion}
	if _, err := Run(context.Background(), src, NewExecClient(""), st, RunOpts{
		Agent: "sami",
	}); err != nil {
		t.Fatalf("integration: %v", err)
	}
}

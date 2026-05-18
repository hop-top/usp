package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/kit/go/core/xdg"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/adapters/opencode"
	"hop.top/usp/internal/uspctxt"
	"hop.top/usp/lineage"
	"hop.top/usp/session"
)

// syncParams carries the user-facing flags to runSync.
type syncParams struct {
	agent       string
	statePath   string
	ctxtServer  string
	dryRun      bool
	perTimeout  int
	cliFilter   string
	projectFlag string
	verbose     bool
}

// runSync orchestrates the bridge run. Wired here so main stays thin
// and the orchestration is testable end-to-end via a fake source.
func runSync(ctx context.Context, p syncParams, errw, outw io.Writer) error {
	st, err := uspctxt.LoadState(p.statePath)
	if err != nil {
		return err
	}

	src := newAdapterSource(p.cliFilter, p.projectFlag)

	var client uspctxt.CtxtClient = uspctxt.NewExecClient(p.ctxtServer)
	if p.dryRun {
		client = dryRunClient{w: errw}
	}
	if p.verbose {
		client = verboseClient{inner: client, w: errw}
	}

	opts := uspctxt.RunOpts{
		Agent:          p.agent,
		PerCallTimeout: time.Duration(p.perTimeout) * time.Second,
		Project:        uspctxt.ProjectOpts{Granularity: uspctxt.SessionGranularity},
	}

	res, err := uspctxt.Run(ctx, src, client, st, opts)
	if err != nil {
		return err
	}

	if !p.dryRun {
		if err := uspctxt.SaveState(p.statePath, st); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}

	fmt.Fprintf(outw, "ingested=%v skipped=%v failed=%v\n",
		res.Ingested, res.Skipped, res.Failed)
	for _, e := range res.Errors {
		fmt.Fprintf(errw, "warn: %v\n", e)
	}
	return nil
}

// adapterSource wraps the live usp adapter set as a SessionSource.
type adapterSource struct {
	clis       []string
	adapters   map[string]session.SessionAdapter
	project    string
	store      *lineage.Store
	nativeByID map[string]string
}

func newAdapterSource(cliFilter, project string) *adapterSource {
	all := map[string]session.SessionAdapter{
		uxp.CLIClaude:   claude.New(),
		uxp.CLICodex:    &codex.Adapter{},
		uxp.CLIOpenCode: opencode.New(),
		uxp.CLIGemini:   &gemini.Adapter{},
	}
	clis := []string{uxp.CLIClaude, uxp.CLICodex, uxp.CLIOpenCode, uxp.CLIGemini}
	if cliFilter != "" {
		if _, ok := all[cliFilter]; ok {
			all = map[string]session.SessionAdapter{cliFilter: all[cliFilter]}
			clis = []string{cliFilter}
		} else {
			// unknown filter — empty source produces no work
			all = map[string]session.SessionAdapter{}
			clis = nil
		}
	}
	src := &adapterSource{
		clis:       clis,
		adapters:   all,
		project:    project,
		nativeByID: map[string]string{},
	}

	// lineage store is best-effort; absence => no cross-CLI roots
	path, err := xdg.StateFile("usp", "sessions.db")
	if err == nil {
		if store, err := lineage.Open(path); err == nil {
			src.store = store
		}
	}
	return src
}

// CLIs satisfies SessionSource.
func (s *adapterSource) CLIs() []string { return s.clis }

// ListSince satisfies SessionSource. Uses the adapter's ListSessions
// then filters by since locally — adapters share no `since` arg.
func (s *adapterSource) ListSince(cli string, since time.Time) ([]session.Session, error) {
	a, ok := s.adapters[cli]
	if !ok {
		return nil, nil
	}
	all, err := a.ListSessions(s.project)
	if err != nil {
		return nil, err
	}
	for _, sess := range all {
		if sess.NativeID != "" {
			s.nativeByID[sess.ID] = sess.NativeID
		}
	}
	if since.IsZero() {
		return all, nil
	}
	out := make([]session.Session, 0, len(all))
	for _, sess := range all {
		if sess.StartedAt.After(since) {
			out = append(out, sess)
		}
	}
	return out, nil
}

// Turns satisfies SessionSource.
func (s *adapterSource) Turns(cli, sessionID string) ([]session.Turn, error) {
	a, ok := s.adapters[cli]
	if !ok {
		return nil, fmt.Errorf("unknown cli %q", cli)
	}
	nativeID := sessionID
	if mapped := s.nativeByID[sessionID]; mapped != "" {
		nativeID = mapped
	}
	ch, err := a.StreamTurns(nativeID)
	if err != nil {
		return nil, err
	}
	var out []session.Turn
	for t := range ch {
		out = append(out, t)
	}
	return out, nil
}

// LineageRoot satisfies SessionSource. Best-effort; absent store
// or missing record returns empty root (treated as no chain).
func (s *adapterSource) LineageRoot(sessionID string) (string, error) {
	if s.store == nil {
		return "", nil
	}
	sess, err := s.store.GetSession(sessionID)
	if err != nil || sess == nil || len(sess.Segments) == 0 {
		return "", nil
	}
	// Lineage root = the logical session ID under which segments hang.
	// usp's lineage model treats sess.ID as the root.
	return sess.ID, nil
}

// dryRunClient logs would-be invocations to a writer.
type dryRunClient struct{ w io.Writer }

func (d dryRunClient) Upsert(_ context.Context, p uspctxt.Projection) error {
	fmt.Fprintf(d.w, "[dry-run] source-key=%s mentions=%q hints=%q bytes=%d\n",
		p.SourceKey, p.MentionsString(), p.HintsString(), len(p.Body))
	return nil
}

// verboseClient wraps another CtxtClient and traces calls.
type verboseClient struct {
	inner uspctxt.CtxtClient
	w     io.Writer
}

func (v verboseClient) Upsert(ctx context.Context, p uspctxt.Projection) error {
	fmt.Fprintf(v.w, "upsert source-key=%s mentions=%q hints=%q bytes=%d\n",
		p.SourceKey, p.MentionsString(), p.HintsString(), len(p.Body))
	return v.inner.Upsert(ctx, p)
}

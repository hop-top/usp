package uspctxt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"hop.top/usp/session"
)

// CtxtBin is the executable invoked by Run. Tests override.
var CtxtBin = "ctxt"

// SessionSource is the shape Run consumes: a list of recent sessions
// per CLI plus a turn fetcher per session. Implemented by the cmd
// layer over the live usp adapter set; stubbed in tests.
type SessionSource interface {
	// CLIs returns the set of CLIs to walk in order.
	CLIs() []string

	// ListSince returns sessions for cli that started at or after
	// since. Implementations should return the result sorted by
	// StartedAt ascending; Run sorts defensively regardless.
	ListSince(cli string, since time.Time) ([]session.Session, error)

	// Turns returns the turns for a session.
	Turns(cli, sessionID string) ([]session.Turn, error)

	// LineageRoot returns the first session ID in a cross-CLI chain
	// when sessionID participates in one. Empty string => not part
	// of a chain. Implementations may return ("", nil) on error;
	// Run treats both as "no lineage info".
	LineageRoot(sessionID string) (string, error)
}

// CtxtClient invokes the ctxt CLI to upsert a Projection.
//
// Default impl (NewExecClient) shells out to ctxt analyze with
// --mentions (primary identity), optional --hints, --source-key
// (secondary dedup), --wait, piping the body via stdin. Tests
// inject a fake.
type CtxtClient interface {
	Upsert(ctx context.Context, p Projection) error
}

// RunOpts configures a single bridge run.
type RunOpts struct {
	// Agent is the producing aps profile id; emitted as @agent.<id>
	// mention on every object.
	Agent string

	// Scope is the logical routing scope; emitted as @scope.<value>
	// mention when non-empty.
	Scope string

	// Project opts forwarded to Project for each session.
	Project ProjectOpts

	// PerCallTimeout caps each ctxt invocation. Zero => 30s.
	PerCallTimeout time.Duration
}

// RunResult summarizes one bridge run.
type RunResult struct {
	Ingested map[string]int
	Skipped  map[string]int
	Failed   map[string]int
	Errors   []error
}

// Run walks src for each CLI, projects sessions started since the
// hwm, and upserts each projection via client. The hwm advances per
// CLI past successfully-ingested sessions only.
//
// Failure semantics (spec §4.5): partial-failure is fine — hwm only
// advances past successes; failed sessions retried next run.
func Run(ctx context.Context, src SessionSource, client CtxtClient, st *State, opts RunOpts) (RunResult, error) {
	if opts.Project.Granularity == "" {
		opts.Project.Granularity = SessionGranularity
	}
	if opts.PerCallTimeout == 0 {
		opts.PerCallTimeout = 30 * time.Second
	}
	if opts.Project.Agent == "" {
		opts.Project.Agent = opts.Agent
	}
	if opts.Project.Scope == "" {
		opts.Project.Scope = opts.Scope
	}

	res := RunResult{
		Ingested: map[string]int{},
		Skipped:  map[string]int{},
		Failed:   map[string]int{},
	}

	for _, cli := range src.CLIs() {
		hwm := st.HWM(cli)
		sessions, err := src.ListSince(cli, hwm)
		if err != nil {
			res.Errors = append(res.Errors,
				fmt.Errorf("list %s: %w", cli, err))
			continue
		}
		SortByStarted(sessions)

		for _, sess := range sessions {
			if !hwm.IsZero() && !sess.StartedAt.After(hwm) {
				// strictly-after hwm; equal => already ingested
				res.Skipped[cli]++
				continue
			}
			if err := ctx.Err(); err != nil {
				return res, err
			}

			turns, err := src.Turns(cli, sess.ID)
			if err != nil {
				res.Failed[cli]++
				res.Errors = append(res.Errors,
					fmt.Errorf("turns %s/%s: %w", cli, sess.ID, err))
				continue
			}
			root, _ := src.LineageRoot(sess.ID)
			po := opts.Project
			po.LineageRoot = root
			proj := Project(sess, turns, po)

			callCtx, cancel := context.WithTimeout(ctx, opts.PerCallTimeout)
			err = client.Upsert(callCtx, proj)
			cancel()
			if err != nil {
				res.Failed[cli]++
				res.Errors = append(res.Errors,
					fmt.Errorf("upsert %s/%s: %w", cli, sess.ID, err))
				continue
			}
			res.Ingested[cli]++
			st.Advance(cli, sess.StartedAt)
		}
	}
	return res, nil
}

// execClient invokes a real ctxt binary.
type execClient struct {
	bin    string
	server string
}

// NewExecClient returns a CtxtClient that shells out to ctxt.
// server may be empty; ctxt defaults to localhost:8080.
func NewExecClient(server string) CtxtClient {
	return &execClient{bin: CtxtBin, server: server}
}

// Upsert pipes p.Body to `ctxt analyze --mentions "<...>"
// [--hints "<...>"] --source-key usp/<id> --wait`. The
// `@usp.session.<id>` mention is the primary identity anchor
// (spec §4.4 / mentions-registry.md); `--source-key` is retained
// as a secondary dedup hint for ctxt's external-id catalog.
// `--hints` is omitted when empty (post-mentions, identity tags
// no longer flow through hints).
func (c *execClient) Upsert(ctx context.Context, p Projection) error {
	args := []string{
		"analyze",
		"--mentions", p.MentionsString(),
	}
	if hs := p.HintsString(); hs != "" {
		args = append(args, "--hints", hs)
	}
	args = append(args, "--source-key", p.SourceKey, "--wait")
	if c.server != "" {
		args = append(args, "--server", c.server)
	}
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Stdin = strings.NewReader(p.Body)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.WaitDelay = 200 * time.Millisecond

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("ctxt: timeout: %w", ctx.Err())
		}
		return fmt.Errorf("ctxt: %w (stderr=%q)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Package api exposes the application API used by CLI and MCP transports.
package api

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"hop.top/kit/go/core/uxp"
	"hop.top/usp/adapters/claude"
	"hop.top/usp/adapters/codex"
	"hop.top/usp/adapters/gemini"
	"hop.top/usp/adapters/opencode"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

// Service is the application boundary for session operations.
type Service struct {
	adapters map[string]session.SessionAdapter
	cache    Cache
}

// Option customizes Service construction.
type Option func(*Service)

// WithCache configures a best-effort cache for read APIs.
func WithCache(cache Cache) Option {
	return func(s *Service) { s.cache = cache }
}

// New constructs a Service over the provided adapters.
func New(adapters map[string]session.SessionAdapter, opts ...Option) *Service {
	s := &Service{adapters: adapters}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// NewDefault constructs a Service wired to the local CLI stores.
func NewDefault(opts ...Option) *Service {
	return New(DefaultAdapters(), opts...)
}

// Close releases resources held by the service.
func (s *Service) Close() error {
	if c, ok := s.cache.(closeCache); ok {
		return c.Close()
	}
	return nil
}

// DefaultAdapters returns the supported local CLI adapters.
func DefaultAdapters() map[string]session.SessionAdapter {
	return map[string]session.SessionAdapter{
		uxp.CLIClaude:   claude.New(),
		uxp.CLICodex:    &codex.Adapter{},
		uxp.CLIOpenCode: opencode.New(),
		uxp.CLIGemini:   &gemini.Adapter{},
	}
}

type ListSessionsRequest struct {
	Project string
	Tool    string
	Since   time.Time
	Limit   int
}

func (s *Service) ListSessions(ctx context.Context, req ListSessionsRequest) ([]session.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	adapters, err := s.filteredAdapters(req.Tool)
	if err != nil {
		return nil, err
	}
	all := s.collectSessions(ctx, adapters, req.Tool, req.Project)
	all = sessionutil.FilterSince(all, req.Since)
	all = sessionutil.SortAndLimit(all, req.Limit)
	return all, nil
}

type SearchSessionsRequest struct {
	Project string
	Tool    string
	Query   string
	Since   time.Time
	Limit   int
}

func (s *Service) SearchSessions(ctx context.Context, req SearchSessionsRequest) ([]session.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	adapters, err := s.filteredAdapters(req.Tool)
	if err != nil {
		return nil, err
	}

	all := s.collectSessions(ctx, adapters, req.Tool, req.Project)
	all = sessionutil.FilterSince(all, req.Since)

	needle := strings.ToLower(req.Query)
	var matched []session.Session
	for _, sess := range all {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		a, ok := adapters[string(sess.CLI)]
		if !ok {
			continue
		}
		ch, err := a.StreamTurns(sess.NativeID)
		if err != nil {
			continue
		}
		for turn := range ch {
			if strings.Contains(strings.ToLower(turn.Content), needle) {
				matched = append(matched, sess)
				for range ch {
				}
				break
			}
		}
	}

	matched = sessionutil.SortAndLimit(matched, req.Limit)
	return matched, nil
}

type ShowSessionRequest struct {
	ID            string
	Tool          string
	Project       string
	Since         time.Time
	IncludeSkills bool
}

type SessionDetail struct {
	Session session.Session
	CLI     string
	Turns   []session.Turn
	Skills  []session.SkillEvent
}

func (s *Service) ShowSession(ctx context.Context, req ShowSessionRequest) (*SessionDetail, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var cached SessionDetail
	if s.cacheGet(ctx, "show-session", req, &cached) {
		return &cached, nil
	}
	adapters, err := s.filteredAdapters(req.Tool)
	if err != nil {
		return nil, err
	}
	opts := sessionutil.ResolveOpts{Project: req.Project, Since: req.Since}
	sess, matchedCLI, adapter, err := sessionutil.ResolveSessionID(
		req.ID, adapters, AdapterOrder(req.ID), opts)
	if err != nil {
		return nil, err
	}

	detail := &SessionDetail{Session: *sess, CLI: matchedCLI}
	ch, err := adapter.StreamTurns(sess.NativeID)
	if err == nil {
		for turn := range ch {
			detail.Turns = append(detail.Turns, turn)
		}
	}

	if req.IncludeSkills {
		if ext, ok := adapter.(session.SkillExtractor); ok {
			if ev, err := ext.ExtractSkills(sess.NativeID); err == nil {
				detail.Skills = ev
			}
		} else {
			detail.Skills = []session.SkillEvent{{
				SessionID:   sess.NativeID,
				CLI:         matchedCLI,
				Timestamp:   sess.StartedAt,
				Unsupported: true,
			}}
		}
	}
	s.cachePut(ctx, "show-session", req, detail)
	return detail, nil
}

type ListSkillEventsRequest struct {
	SessionID string
	Tool      string
	Project   string
	Name      string
	Since     time.Time
	Until     time.Time
}

func (s *Service) ListSkillEvents(ctx context.Context, req ListSkillEventsRequest) ([]session.SkillEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var cached []session.SkillEvent
	if s.cacheGet(ctx, "list-skills", req, &cached) {
		return cached, nil
	}
	adapters, err := s.filteredAdapters(req.Tool)
	if err != nil {
		return nil, err
	}

	type target struct {
		sess session.Session
		cli  string
		a    session.SessionAdapter
	}
	var targets []target

	if req.SessionID != "" {
		opts := sessionutil.ResolveOpts{Project: req.Project, Since: req.Since}
		sess, cli, adapter, err := sessionutil.ResolveSessionID(
			req.SessionID, adapters, AdapterOrder(req.SessionID), opts)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target{sess: *sess, cli: cli, a: adapter})
	} else {
		sessions := s.collectSessions(ctx, adapters, req.Tool, req.Project)
		for _, sess := range sessions {
			cli := string(sess.CLI)
			adapter, ok := adapters[cli]
			if !ok {
				continue
			}
			targets = append(targets, target{sess: sess, cli: cli, a: adapter})
		}
	}

	var out []session.SkillEvent
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !req.Since.IsZero() && t.sess.StartedAt.Before(req.Since) {
			continue
		}
		if !req.Until.IsZero() && t.sess.StartedAt.After(req.Until) {
			continue
		}
		ext, ok := t.a.(session.SkillExtractor)
		if !ok {
			out = append(out, session.SkillEvent{
				SessionID:   t.sess.NativeID,
				CLI:         t.cli,
				Timestamp:   t.sess.StartedAt,
				Unsupported: true,
			})
			continue
		}
		events, err := ext.ExtractSkills(t.sess.NativeID)
		if err != nil {
			continue
		}
		for _, event := range events {
			if !req.Since.IsZero() && event.Timestamp.Before(req.Since) {
				continue
			}
			if !req.Until.IsZero() && event.Timestamp.After(req.Until) {
				continue
			}
			if req.Name != "" {
				if event.Unsupported {
					continue
				}
				if !strings.Contains(strings.ToLower(event.Name), strings.ToLower(req.Name)) {
					continue
				}
			}
			out = append(out, event)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	s.cachePut(ctx, "list-skills", req, out)
	return out, nil
}

func (s *Service) collectSessions(
	ctx context.Context,
	adapters map[string]session.SessionAdapter,
	tool string,
	project string,
) []session.Session {
	req := struct {
		Tool    string `json:"tool"`
		Project string `json:"project"`
	}{Tool: tool, Project: project}
	var cached []session.Session
	if s.cacheGet(ctx, "collect-sessions", req, &cached) {
		return cached
	}
	all := sessionutil.CollectSessions(adapters, project)
	s.cachePut(ctx, "collect-sessions", req, all)
	return all
}

func (s *Service) filteredAdapters(tool string) (map[string]session.SessionAdapter, error) {
	adapters := sessionutil.FilterAdapters(s.adapters, tool)
	if adapters == nil {
		return nil, fmt.Errorf("unknown CLI %q", tool)
	}
	return adapters, nil
}

// ID-format regexes for adapter priority hinting.
var (
	reUUIDv4   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	reUUIDv7   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	reOpenCode = regexp.MustCompile(`^ses_[a-z0-9]{26}$`)
)

// AdapterOrder returns adapter names to try, prioritised by ID format.
func AdapterOrder(id string) []string {
	switch {
	case reUUIDv4.MatchString(id):
		return []string{uxp.CLIClaude, uxp.CLICodex, uxp.CLIOpenCode, uxp.CLIGemini}
	case reUUIDv7.MatchString(id):
		return []string{uxp.CLICodex, uxp.CLIClaude, uxp.CLIOpenCode, uxp.CLIGemini}
	case reOpenCode.MatchString(id):
		return []string{uxp.CLIOpenCode, uxp.CLIClaude, uxp.CLICodex, uxp.CLIGemini}
	default:
		return []string{uxp.CLIClaude, uxp.CLICodex, uxp.CLIOpenCode, uxp.CLIGemini}
	}
}

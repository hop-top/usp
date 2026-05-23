package api

import (
	"context"
	"sort"
	"strings"
	"time"

	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

type SessionListItem struct {
	Session session.Session
	Actions string
}

func (s *Service) ListSessionItems(
	ctx context.Context,
	req ListSessionsRequest,
) ([]SessionListItem, error) {
	sessions, err := s.ListSessions(ctx, req)
	if err != nil {
		return nil, err
	}
	items := make([]SessionListItem, 0, len(sessions))
	for _, sess := range sessions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		items = append(items, SessionListItem{
			Session: sess,
			Actions: s.sessionActions(ctx, sess),
		})
	}
	return items, nil
}

type FindSessionItemsRequest struct {
	Project string
	CLI     string
	Filter  string
	Since   time.Time
	Limit   int
}

func (s *Service) FindSessionItems(
	ctx context.Context,
	req FindSessionItemsRequest,
) ([]SessionListItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	adapters, err := s.filteredAdapters(req.CLI)
	if err != nil {
		return nil, err
	}
	matcher := newContentMatcher(req.Filter)
	sessions := s.collectSessions(ctx, adapters, req.CLI, req.Project)
	sessions = sessionutil.FilterSince(sessions, req.Since)

	// Sort upfront so we can stop streaming turns once req.Limit matches
	// have been collected. Output stays identical to the prior
	// scan-everything-then-sort approach: stable sort preserves tie order,
	// and matched sessions are appended in already-sorted order.
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})

	capacity := len(sessions)
	if req.Limit > 0 && req.Limit < capacity {
		capacity = req.Limit
	}
	items := make([]SessionListItem, 0, capacity)
	for _, sess := range sessions {
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
		var turns []session.Turn
		matched := false
		for turn := range ch {
			turns = append(turns, turn)
			if matcher.MatchTurn(turn) {
				matched = true
			}
		}
		if !matched {
			continue
		}
		items = append(items, SessionListItem{
			Session: sess,
			Actions: SummarizeTurns(turns),
		})
		if req.Limit > 0 && len(items) >= req.Limit {
			break
		}
	}
	return items, nil
}

func (s *Service) sessionActions(ctx context.Context, sess session.Session) string {
	req := struct {
		CLI       string    `json:"cli"`
		NativeID  string    `json:"native_id"`
		TurnCount int       `json:"turn_count"`
		StartedAt time.Time `json:"started_at"`
	}{
		CLI:       string(sess.CLI),
		NativeID:  sess.NativeID,
		TurnCount: sess.TurnCount,
		StartedAt: sess.StartedAt,
	}
	var cached string
	if s.cacheGet(ctx, "session-actions", req, &cached) {
		return cached
	}

	a, ok := s.adapters[string(sess.CLI)]
	if !ok {
		return ""
	}
	ch, err := a.StreamTurns(sess.NativeID)
	if err != nil {
		return ""
	}
	var turns []session.Turn
	for turn := range ch {
		turns = append(turns, turn)
	}
	actions := SummarizeTurns(turns)
	s.cachePut(ctx, "session-actions", req, actions)
	return actions
}

type contentMatcher struct {
	raw     string
	value   string
	compact string
}

func newContentMatcher(filter string) contentMatcher {
	raw := strings.ToLower(strings.TrimSpace(filter))
	value := raw
	if before, after, ok := strings.Cut(raw, ":"); ok && before == "commit" {
		value = strings.TrimSpace(after)
	}
	return contentMatcher{
		raw:     raw,
		value:   value,
		compact: compactSearchText(value),
	}
}

func (m contentMatcher) MatchTurn(turn session.Turn) bool {
	if m.raw == "" {
		return true
	}
	var b strings.Builder
	b.Grow(len(turn.Content) + 32*len(turn.ToolCalls))
	b.WriteString(turn.Content)
	for _, call := range turn.ToolCalls {
		b.WriteByte(' ')
		b.WriteString(call.Name)
		b.WriteByte(' ')
		b.WriteString(call.Input)
		b.WriteByte(' ')
		b.WriteString(call.Output)
	}
	text := b.String()
	lower := strings.ToLower(text)
	if strings.Contains(lower, m.raw) || strings.Contains(lower, m.value) {
		return true
	}
	return m.compact != "" && strings.Contains(compactSearchText(text), m.compact)
}

func compactSearchText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func SummarizeTurns(turns []session.Turn) string {
	if s := finalAssistantSentence(turns); s != "" {
		return truncateText(s, 120)
	}

	cats := map[string]bool{}
	var topic string
	for _, turn := range turns {
		if topic == "" && turn.Role == session.RoleUser {
			topic = truncateText(cleanOneLine(turn.Content), 48)
		}
		for _, call := range turn.ToolCalls {
			switch categorizeTool(call.Name) {
			case "research":
				cats["researched code"] = true
			case "edit":
				cats["changed files"] = true
			case "test":
				cats["ran tests"] = true
			case "build":
				cats["built binaries"] = true
			case "git":
				cats["updated git state"] = true
			}
		}
	}

	parts := make([]string, 0, len(cats)+1)
	for part := range cats {
		parts = append(parts, part)
	}
	sort.Strings(parts)
	if topic != "" {
		parts = append([]string{"discussed " + topic}, parts...)
	}
	if len(parts) == 0 {
		return ""
	}
	return truncateText(strings.Join(parts, "; "), 120)
}

func finalAssistantSentence(turns []session.Turn) string {
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		if turn.Role != session.RoleAssistant {
			continue
		}
		text := cleanOneLine(turn.Content)
		if text == "" {
			continue
		}
		for _, sep := range []string{". ", "\n", " - ", "; "} {
			if idx := strings.Index(text, sep); idx > 0 {
				text = text[:idx+1]
				break
			}
		}
		return strings.TrimSpace(strings.TrimPrefix(text, "Summary:"))
	}
	return ""
}

func categorizeTool(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "grep") || strings.Contains(n, "read") ||
		strings.Contains(n, "search") || strings.Contains(n, "find"):
		return "research"
	case strings.Contains(n, "patch") || strings.Contains(n, "edit") ||
		strings.Contains(n, "write"):
		return "edit"
	case strings.Contains(n, "test"):
		return "test"
	case strings.Contains(n, "build"):
		return "build"
	case strings.Contains(n, "git"):
		return "git"
	default:
		return ""
	}
}

func cleanOneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return strings.TrimSpace(s[:max-1]) + "…"
}

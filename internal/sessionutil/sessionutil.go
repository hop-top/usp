// Package sessionutil provides shared session filtering, sorting,
// and parsing helpers used by both cmd/usp and e2e tests.
package sessionutil

import (
	"fmt"
	"sort"
	"time"

	"hop.top/usp/session"
)

// FilterSince returns sessions started at or after since.
// Returns all sessions if since is zero.
func FilterSince(ss []session.Session, since time.Time) []session.Session {
	if since.IsZero() {
		return ss
	}
	var filtered []session.Session
	for _, s := range ss {
		if !s.StartedAt.Before(since) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// SortAndLimit sorts sessions by StartedAt descending and
// truncates to limit. Limit <= 0 means no truncation.
func SortAndLimit(ss []session.Session, limit int) []session.Session {
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].StartedAt.After(ss[j].StartedAt)
	})
	if limit > 0 && len(ss) > limit {
		ss = ss[:limit]
	}
	return ss
}

// FilterAdapters returns only the named adapter, or all if tool
// is empty. Returns nil if tool is not found.
func FilterAdapters(
	all map[string]session.SessionAdapter, tool string,
) map[string]session.SessionAdapter {
	if tool == "" {
		return all
	}
	a, ok := all[tool]
	if !ok {
		return nil
	}
	return map[string]session.SessionAdapter{tool: a}
}

// CollectSessions fans out to all adapters and merges results.
func CollectSessions(
	adapters map[string]session.SessionAdapter, project string,
) []session.Session {
	var all []session.Session
	for _, a := range adapters {
		sessions, err := a.ListSessions(project)
		if err != nil {
			continue
		}
		all = append(all, sessions...)
	}
	return all
}

// ParseSince parses duration shorthand (7d, 24h, 30m) or date
// (2026-04-01). Returns zero time if input is empty.
func ParseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if len(s) >= 2 {
		unit := s[len(s)-1]
		val := s[:len(s)-1]
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			switch unit {
			case 'd':
				return time.Now().Add(
					-time.Duration(n) * 24 * time.Hour), nil
			case 'h':
				return time.Now().Add(
					-time.Duration(n) * time.Hour), nil
			case 'm':
				return time.Now().Add(
					-time.Duration(n) * time.Minute), nil
			}
		}
	}
	for _, layout := range []string{
		"2006-01-02",
		"2006-01-02T15:04:05",
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf(
		"expected duration (7d, 24h) or date (2026-04-01), got %q", s)
}

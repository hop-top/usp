// Package sessionutil provides shared session filtering, sorting,
// and parsing helpers used by both cmd/usp and e2e tests.
package sessionutil

import (
	"fmt"
	"sort"
	"strings"
	"time"

	kitutil "hop.top/kit/go/core/util"
	"hop.top/usp/internal/id"
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

// FilterAdapters returns only the named CLI adapter, or all if cliName
// is empty. Returns nil if cliName is not found.
func FilterAdapters(
	all map[string]session.SessionAdapter, cliName string,
) map[string]session.SessionAdapter {
	if cliName == "" {
		return all
	}
	a, ok := all[cliName]
	if !ok {
		return nil
	}
	return map[string]session.SessionAdapter{cliName: a}
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

// ResolveOpts narrows prefix matching by project and/or time range.
type ResolveOpts struct {
	Project string
	Since   time.Time
}

// ResolveSessionID resolves a session id to an exact match. Accepts
// either the canonical TypeID (sess_…), a native CLI session id
// (UUID, ses_…, tag), or a partial prefix of either form.
// Returns the matched session, adapter name, and adapter.
// If the prefix is ambiguous, returns an error listing matches.
func ResolveSessionID(
	input string,
	adapters map[string]session.SessionAdapter,
	order []string,
	opts ...ResolveOpts,
) (*session.Session, string, session.SessionAdapter, error) {
	var opt ResolveOpts
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Native fast path: ask each adapter to fetch by native id.
	// Adapter file/db lookups use the native id, never the TypeID.
	if !id.IsTypeID(input) {
		for _, name := range order {
			a, ok := adapters[name]
			if !ok {
				continue
			}
			s, err := a.GetSession(input)
			if err == nil && s != nil && s.NativeID == input {
				return s, name, a, nil
			}
		}
	}

	// TypeID or prefix path: enumerate sessions and match against
	// either ID (TypeID) or NativeID. Prefix-match keeps T-0061
	// UX (native UUID prefix lookup) working.
	type match struct {
		sess *session.Session
		cli  string
		a    session.SessionAdapter
	}
	var matches []match
	for _, name := range order {
		a, ok := adapters[name]
		if !ok {
			continue
		}
		sessions, err := a.ListSessions(opt.Project)
		if err != nil {
			continue
		}
		for i := range sessions {
			s := &sessions[i]
			if !sessionMatches(s, input) {
				continue
			}
			if !opt.Since.IsZero() && s.StartedAt.Before(opt.Since) {
				continue
			}
			matches = append(matches, match{s, name, a})
		}
	}

	switch len(matches) {
	case 0:
		return nil, "", nil, fmt.Errorf("session %q not found", input)
	case 1:
		return matches[0].sess, matches[0].cli, matches[0].a, nil
	default:
		// Dedupe: same session can match by both ID and NativeID
		// only when input is a full id, in which case there's
		// genuinely one match per (cli, native) pair.
		var ids []string
		for _, m := range matches {
			ids = append(ids, fmt.Sprintf(
				"  %s (%s, native=%s)", m.sess.ID, m.cli, m.sess.NativeID))
		}
		return nil, "", nil, fmt.Errorf(
			"prefix %q is ambiguous (%d matches):\n%s",
			input, len(matches), strings.Join(ids, "\n"))
	}
}

// sessionMatches returns true when the input prefix-matches either
// the TypeID or the native id. Exact equality is just a length-26+
// prefix match, so this also covers full-id lookups.
func sessionMatches(s *session.Session, input string) bool {
	if input == "" {
		return false
	}
	if strings.HasPrefix(s.ID, input) {
		return true
	}
	if s.NativeID != "" && strings.HasPrefix(s.NativeID, input) {
		return true
	}
	return false
}

// ParseSince parses kit's shared date filter syntax (7d, 24h, 30m,
// "2 weeks ago", 2026-04-01). Returns zero time if input is empty.
func ParseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := kitutil.ParseSince(s)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"expected date filter (e.g. 10m, 7d, 2 weeks ago, 2026-04-01), got %q: %w",
			s, err)
	}
	return t, nil
}

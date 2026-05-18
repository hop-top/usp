package main

import (
	"regexp"
	"testing"
	"time"

	"hop.top/kit/go/console/output"
	"hop.top/usp/session"
)

// rfc3339Re matches a reasonable RFC3339 timestamp (with or without
// fractional seconds and a Z/offset). Sufficient to distinguish from
// the table-mode "Nm ago" / "1h ago" / "2d ago" forms.
var rfc3339Re = regexp.MustCompile(
	`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`,
)

func TestToRows_ISO8601ForJSONAndYAML(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 30, 45, 0, time.UTC)
	ss := []session.Session{{
		ID:         "abc",
		NativeID:   "abc-native",
		CLI:        "claude",
		ProjectCwd: "/tmp/proj",
		StartedAt:  now,
		TurnCount:  3,
	}}

	for _, fmt := range []output.Format{output.JSON, output.YAML} {
		rows := toRows(ss, fmt)
		if len(rows) != 1 {
			t.Fatalf("%s: rows = %d, want 1", fmt, len(rows))
		}
		if !rfc3339Re.MatchString(rows[0].Started) {
			t.Errorf("%s: Started = %q, want RFC3339", fmt, rows[0].Started)
		}
	}
}

func TestToRows_RelativeForTable(t *testing.T) {
	now := time.Now().Add(-3 * time.Hour)
	ss := []session.Session{{StartedAt: now}}
	rows := toRows(ss, output.Table)
	if rfc3339Re.MatchString(rows[0].Started) {
		t.Errorf("table: Started should not be RFC3339, got %q", rows[0].Started)
	}
}

func TestTimestampForFormat(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 30, 45, 0, time.UTC)

	if got := timestampForFormat(now, output.JSON); !rfc3339Re.MatchString(got) {
		t.Errorf("JSON: got %q, want RFC3339", got)
	}
	if got := timestampForFormat(now, output.YAML); !rfc3339Re.MatchString(got) {
		t.Errorf("YAML: got %q, want RFC3339", got)
	}
	if got := timestampForFormat(now, output.Table); rfc3339Re.MatchString(got) {
		t.Errorf("Table: got %q, should not be RFC3339", got)
	}
	if got := timestampForFormat(time.Time{}, output.JSON); got != "" {
		t.Errorf("zero time: got %q, want empty", got)
	}
}

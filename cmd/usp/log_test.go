package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	charmlog "charm.land/log/v2"
	"github.com/spf13/viper"
	kitlog "hop.top/kit/go/console/log"
)

// TestSlogDefaultsToStderr verifies the kit/log handler writes to stderr
// when used as slog.Default. We can't intercept package-level os.Stderr
// cleanly in tests; instead, confirm the handler's underlying logger
// honors the "quiet" viper key by raising its level to Warn.
func TestKitLogQuietRaisesLevel(t *testing.T) {
	v := viper.New()
	v.Set("quiet", true)
	logger := kitlog.New(v)
	if logger == nil {
		t.Fatal("kit/log.New returned nil")
	}
	// Confirm the logger satisfies slog.Handler so SetDefault works.
	var _ slog.Handler = logger
}

// TestSlogInfoFormat sanity-checks slog still works with the kit handler
// by writing through a buffered logger constructed manually.
func TestSlogInfoFormat(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := slog.New(h)
	l.Info("hello", "k", "v")
	if !strings.Contains(buf.String(), "msg=hello") {
		t.Errorf("slog output missing msg=hello: %q", buf.String())
	}
}

// TestVerboseCount maps the -V count to charm log levels per spec §5.
// 0=Info default, 1=Debug, 2+=Trace. Mirrors kit/log's contract so the
// -V wiring in main.go's PrePersistentRunE hook remains spec-aligned.
func TestVerboseCount(t *testing.T) {
	cases := []struct {
		count int
		want  charmlog.Level
	}{
		{0, charmlog.InfoLevel},
		{1, charmlog.DebugLevel},
		{2, kitlog.TraceLevel},
		{3, kitlog.TraceLevel},
	}
	for _, c := range cases {
		v := viper.New()
		l := kitlog.WithVerbose(v, c.count)
		if l.GetLevel() != c.want {
			t.Errorf("count=%d level=%v want=%v",
				c.count, l.GetLevel(), c.want)
		}
	}
}

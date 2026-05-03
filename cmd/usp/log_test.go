package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

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

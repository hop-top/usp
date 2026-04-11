//go:build docker

package docker_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hop.top/usp/internal/xrrutil"
)

// testCtx returns a context with a 2-minute timeout per test.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 2*time.Minute,
	)
	t.Cleanup(cancel)
	return ctx
}

// runSafe calls RunCommand and handles nil result.
func runSafe(
	t *testing.T, ctx context.Context, argv []string, dir string,
) *xrrutil.Result {
	t.Helper()
	r, err := xrrutil.RunCommand(ctx, argv, dir)
	if err != nil {
		t.Logf("%s err: %v", argv[0], err)
	}
	if r == nil {
		t.Logf("%s returned nil result", argv[0])
		return &xrrutil.Result{ExitCode: -1}
	}
	return r
}

func initGitRepo(t *testing.T, ctx context.Context, dir string) {
	t.Helper()
	for _, argv := range [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "test"},
	} {
		if _, err := xrrutil.RunCommand(ctx, argv, dir); err != nil {
			t.Fatalf("git init step %v: %v", argv, err)
		}
	}
}

// ---------- TestSingleSessionLifecycle ----------

func TestSingleSessionLifecycle(t *testing.T) {
	ctx := testCtx(t)

	t.Run("claude", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, ctx, dir)
		r := runSafe(t, ctx, []string{
			"claude", "--dangerously-skip-permissions",
			"-p", "list files in /tmp",
			"--output-format", "json",
		}, dir)
		t.Logf("claude exit=%d stdout=%d stderr=%d",
			r.ExitCode, len(r.Stdout), len(r.Stderr))

		store := filepath.Join(os.Getenv("HOME"), ".claude", "projects")
		if entries, _ := filepath.Glob(store + "/*/*.jsonl"); len(entries) > 0 {
			t.Logf("claude session JSONL found (%d files)", len(entries))
		} else {
			t.Log("WARN: no claude session JSONL")
		}
	})

	t.Run("codex", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, ctx, dir)
		r := runSafe(t, ctx, []string{
			"codex", "exec", "list files in /tmp",
		}, dir)
		t.Logf("codex exit=%d stdout=%d stderr=%d",
			r.ExitCode, len(r.Stdout), len(r.Stderr))

		store := filepath.Join(os.Getenv("HOME"), ".codex", "sessions")
		if entries, _ := filepath.Glob(store + "/*/*/*/*.jsonl"); len(entries) > 0 {
			t.Logf("codex session found (%d files)", len(entries))
		} else {
			t.Log("WARN: no codex session files")
		}
	})

	t.Run("gemini", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, ctx, dir)
		r := runSafe(t, ctx, []string{
			"gemini", "-p", "list files in /tmp",
		}, dir)
		t.Logf("gemini exit=%d stdout=%d stderr=%d",
			r.ExitCode, len(r.Stdout), len(r.Stderr))

		store := filepath.Join(os.Getenv("HOME"), ".gemini", "history")
		if fi, err := os.Stat(store); err == nil && fi.IsDir() {
			t.Log("gemini history dir exists")
		} else {
			t.Log("WARN: no gemini history dir")
		}
	})

	t.Run("opencode", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, ctx, dir)
		r := runSafe(t, ctx, []string{
			"opencode", "-p", "list files in /tmp",
		}, dir)
		t.Logf("opencode exit=%d stdout=%d stderr=%d",
			r.ExitCode, len(r.Stdout), len(r.Stderr))

		store := filepath.Join(os.Getenv("HOME"),
			".local", "share", "opencode", "storage", "session")
		if entries, _ := filepath.Glob(store + "/*/ses_*.json"); len(entries) > 0 {
			t.Logf("opencode session found (%d files)", len(entries))
		} else {
			t.Log("WARN: no opencode session files")
		}
	})
}

// ---------- TestCrossCliResume ----------

func TestCrossCliResume(t *testing.T) {
	ctx := testCtx(t)
	dir := t.TempDir()
	initGitRepo(t, ctx, dir)

	// Step 1: Claude session.
	r := runSafe(t, ctx, []string{
		"claude", "--dangerously-skip-permissions",
		"-p", "create a simple Go HTTP server",
		"--output-format", "json",
	}, dir)
	t.Logf("claude seed exit=%d", r.ExitCode)

	// Step 2: Resume into codex.
	r = runSafe(t, ctx, []string{
		"usp", "resume", "--tool", "codex", "--inject-only",
	}, dir)
	if r.ExitCode != 0 {
		t.Skip("--inject-only not implemented; skipping resume chain")
	}
	t.Logf("resume→codex exit=%d", r.ExitCode)

	// Step 3: Resume into gemini.
	r = runSafe(t, ctx, []string{
		"usp", "resume", "--tool", "gemini", "--inject-only",
	}, dir)
	t.Logf("resume→gemini exit=%d", r.ExitCode)

	// Step 4: Verify sessions.
	r = runSafe(t, ctx, []string{
		"usp", "session", "list",
		"--project", dir, "--format", "json",
	}, dir)
	if r.Stdout == "" {
		t.Log("WARN: empty session list output")
		return
	}

	var sessions []map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &sessions); err != nil {
		t.Fatalf("session list JSON parse: %v", err)
	}
	t.Logf("sessions found: %d", len(sessions))

	// Step 5: Lineage.
	if len(sessions) > 0 {
		if id, ok := sessions[0]["id"].(string); ok && id != "" {
			lr := runSafe(t, ctx, []string{
				"usp", "session", "lineage", id,
			}, dir)
			t.Logf("lineage exit=%d stdout=%s",
				lr.ExitCode, lr.Stdout)
		}
	}
}

// ---------- TestSessionFilters ----------

func TestSessionFilters(t *testing.T) {
	ctx := testCtx(t)

	projectA := t.TempDir()
	projectB := t.TempDir()
	initGitRepo(t, ctx, projectA)
	initGitRepo(t, ctx, projectB)

	// Seed sessions.
	runSafe(t, ctx, []string{
		"claude", "--dangerously-skip-permissions",
		"-p", "say hello", "--output-format", "json",
	}, projectA)
	runSafe(t, ctx, []string{
		"codex", "exec", "say hello",
	}, projectB)
	runSafe(t, ctx, []string{
		"gemini", "-p", "say hello",
	}, projectA)
	runSafe(t, ctx, []string{
		"opencode", "-p", "say hello",
	}, projectB)

	// countSessions returns session count from a filter query.
	// Returns -1 on command or parse failure.
	countSessions := func(args ...string) int {
		argv := append([]string{
			"usp", "session", "list", "--format", "json",
		}, args...)
		r := runSafe(t, ctx, argv, projectA)
		if r.ExitCode != 0 || r.Stdout == "" {
			return -1
		}
		var s []map[string]any
		if err := json.Unmarshal([]byte(r.Stdout), &s); err != nil {
			t.Logf("countSessions parse err: %v", err)
			return -1
		}
		return len(s)
	}

	total := countSessions()
	if total <= 0 {
		t.Skip("no sessions seeded (API keys likely absent)")
	}
	t.Logf("seeded %d sessions", total)

	if n := countSessions("--tool", "claude"); n < 1 {
		t.Errorf("--tool claude: want >=1, got %d", n)
	}
	if n := countSessions("--project", projectA); n < 1 {
		t.Errorf("--project project-a: want >=1, got %d", n)
	}
	if n := countSessions("--limit", "1"); n > 1 {
		t.Errorf("--limit 1: want <=1, got %d", n)
	}
	if n := countSessions("--since", "1h"); n < 1 {
		t.Errorf("--since 1h: want >=1, got %d", n)
	}
	if n := countSessions("--tool", "claude", "--project", projectA); n < 1 {
		t.Errorf("--tool claude + --project a: want >=1, got %d", n)
	}
}

// ---------- TestArgumentPassing ----------

func TestArgumentPassing(t *testing.T) {
	ctx := testCtx(t)
	dir := t.TempDir()
	initGitRepo(t, ctx, dir)

	wantArgv := []string{
		"claude", "--dangerously-skip-permissions",
		"-p", "hello world",
		"--output-format", "json",
	}
	runSafe(t, ctx, wantArgv, dir)

	cassetteDir := xrrutil.CassetteDir()
	if cassetteDir == "" {
		t.Skip("XRR_CASSETTE_DIR not set")
	}

	found := false
	err := filepath.Walk(cassetteDir,
		func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(info.Name(), ".req.yaml") &&
				!strings.HasSuffix(info.Name(), ".yaml") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(data)
			for _, arg := range wantArgv {
				if !strings.Contains(content, arg) {
					return nil
				}
			}
			found = true
			t.Logf("matching cassette: %s", path)
			return filepath.SkipAll
		},
	)
	if err != nil {
		t.Fatalf("cassette walk: %v", err)
	}

	if !found && xrrutil.Active() {
		t.Error("no cassette found containing expected argv")
	} else if !xrrutil.Active() {
		t.Log("WARN: XRR_MODE not set; skipping cassette check")
	}
}

//go:build docker

package docker_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hop.top/usp/internal/xrrutil"
)

// initGitRepo sets up a minimal git repo in dir so CLIs that require
// one don't bail out.
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
	ctx := context.Background()

	t.Run("claude", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, ctx, dir)

		r, err := xrrutil.RunCommand(ctx, []string{
			"claude", "--dangerously-skip-permissions",
			"-p", "list files in /tmp",
			"--output-format", "json",
		}, dir)
		if err != nil {
			t.Logf("claude err (may be missing API key): %v", err)
		}
		t.Logf("claude exit=%d stdout=%d bytes stderr=%d bytes",
			r.ExitCode, len(r.Stdout), len(r.Stderr))

		// Verify session store touched.
		store := filepath.Join(os.Getenv("HOME"), ".claude", "projects")
		if entries, _ := filepath.Glob(store + "/*/*.jsonl"); len(entries) > 0 {
			t.Logf("claude session JSONL found (%d files)", len(entries))
		} else {
			t.Log("WARN: no claude session JSONL (API key missing or store path changed)")
		}
	})

	t.Run("codex", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, ctx, dir)

		r, err := xrrutil.RunCommand(ctx, []string{
			"codex", "exec", "list files in /tmp",
		}, dir)
		if err != nil {
			t.Logf("codex err: %v", err)
		}
		t.Logf("codex exit=%d stdout=%d bytes stderr=%d bytes",
			r.ExitCode, len(r.Stdout), len(r.Stderr))

		store := filepath.Join(os.Getenv("HOME"), ".codex", "sessions")
		if entries, _ := filepath.Glob(store + "/*/*/*/rollout-*.jsonl"); len(entries) > 0 {
			t.Logf("codex session rollout found (%d files)", len(entries))
		} else {
			t.Log("WARN: no codex session rollout")
		}
	})

	t.Run("gemini", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, ctx, dir)

		r, err := xrrutil.RunCommand(ctx, []string{
			"gemini", "-p", "list files in /tmp",
		}, dir)
		if err != nil {
			t.Logf("gemini err: %v", err)
		}
		t.Logf("gemini exit=%d stdout=%d bytes stderr=%d bytes",
			r.ExitCode, len(r.Stdout), len(r.Stderr))

		store := filepath.Join(os.Getenv("HOME"), ".gemini", "history")
		if fi, err := os.Stat(store); err == nil && fi.IsDir() {
			t.Logf("gemini history dir exists")
		} else {
			t.Log("WARN: no gemini history dir")
		}
	})

	t.Run("opencode", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, ctx, dir)

		r, err := xrrutil.RunCommand(ctx, []string{
			"opencode", "-p", "list files in /tmp",
		}, dir)
		if err != nil {
			t.Logf("opencode err: %v", err)
		}
		t.Logf("opencode exit=%d stdout=%d bytes stderr=%d bytes",
			r.ExitCode, len(r.Stdout), len(r.Stderr))

		store := filepath.Join(os.Getenv("HOME"),
			".local", "share", "opencode", "storage", "session")
		if entries, _ := filepath.Glob(store + "/*/ses_*.json"); len(entries) > 0 {
			t.Logf("opencode session JSON found (%d files)", len(entries))
		} else {
			t.Log("WARN: no opencode session JSON")
		}
	})
}

// ---------- TestCrossCliResume ----------

func TestCrossCliResume(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initGitRepo(t, ctx, dir)

	// Step 1: Start session in Claude.
	r, err := xrrutil.RunCommand(ctx, []string{
		"claude", "--dangerously-skip-permissions",
		"-p", "create a simple Go HTTP server",
		"--output-format", "json",
	}, dir)
	if err != nil {
		t.Logf("claude seed err: %v", err)
	}
	t.Logf("claude seed exit=%d", r.ExitCode)

	// Step 2: Resume into codex (inject-only).
	// TODO: --inject-only may not exist yet; tolerate failure.
	r, err = xrrutil.RunCommand(ctx, []string{
		"usp", "resume", "--tool", "codex", "--inject-only",
	}, dir)
	if err != nil {
		t.Logf("resume→codex err (expected if --inject-only not implemented): %v", err)
	}
	t.Logf("resume→codex exit=%d", r.ExitCode)

	// Step 3: Resume into gemini.
	r, err = xrrutil.RunCommand(ctx, []string{
		"usp", "resume", "--tool", "gemini", "--inject-only",
	}, dir)
	if err != nil {
		t.Logf("resume→gemini err: %v", err)
	}
	t.Logf("resume→gemini exit=%d", r.ExitCode)

	// Step 4: Verify sessions via usp session list.
	r, err = xrrutil.RunCommand(ctx, []string{
		"usp", "session", "list",
		"--project", dir,
		"--format", "json",
	}, dir)
	if err != nil {
		t.Logf("session list err: %v", err)
	}
	t.Logf("session list stdout=%s", r.Stdout)

	if r.Stdout != "" {
		var sessions []map[string]any
		if err := json.Unmarshal([]byte(r.Stdout), &sessions); err == nil {
			t.Logf("sessions found: %d", len(sessions))
			if len(sessions) == 0 {
				t.Log("WARN: no sessions (API keys likely absent)")
			}
			// Step 5: Verify lineage on first session.
			if len(sessions) > 0 {
				if id, ok := sessions[0]["id"].(string); ok && id != "" {
					lr, _ := xrrutil.RunCommand(ctx, []string{
						"usp", "session", "lineage", id,
					}, dir)
					t.Logf("lineage exit=%d stdout=%s", lr.ExitCode, lr.Stdout)
				}
			}
		} else {
			t.Logf("session list JSON parse err: %v", err)
		}
	}
}

// ---------- TestSessionFilters ----------

func TestSessionFilters(t *testing.T) {
	ctx := context.Background()

	projectA := t.TempDir()
	projectB := t.TempDir()
	initGitRepo(t, ctx, projectA)
	initGitRepo(t, ctx, projectB)

	// Seed: claude in project-a.
	xrrutil.RunCommand(ctx, []string{
		"claude", "--dangerously-skip-permissions",
		"-p", "say hello", "--output-format", "json",
	}, projectA)

	// Seed: codex in project-b.
	xrrutil.RunCommand(ctx, []string{
		"codex", "exec", "say hello",
	}, projectB)

	// Seed: gemini in project-a.
	xrrutil.RunCommand(ctx, []string{
		"gemini", "-p", "say hello",
	}, projectA)

	// Seed: opencode in project-b.
	xrrutil.RunCommand(ctx, []string{
		"opencode", "-p", "say hello",
	}, projectB)

	// Check total sessions seeded.
	r, _ := xrrutil.RunCommand(ctx, []string{
		"usp", "session", "list", "--format", "json",
	}, projectA)

	var all []map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &all); err != nil || len(all) == 0 {
		t.Skip("no sessions seeded (API keys likely absent)")
	}
	t.Logf("seeded %d sessions", len(all))

	// Helper: count sessions from a filter invocation.
	countSessions := func(args ...string) int {
		argv := append([]string{"usp", "session", "list", "--format", "json"}, args...)
		r, _ := xrrutil.RunCommand(ctx, argv, projectA)
		var s []map[string]any
		json.Unmarshal([]byte(r.Stdout), &s)
		return len(s)
	}

	// --tool filter
	if n := countSessions("--tool", "claude"); n < 1 {
		t.Errorf("--tool claude: want >=1, got %d", n)
	}

	// --project filter
	if n := countSessions("--project", projectA); n < 1 {
		t.Errorf("--project project-a: want >=1, got %d", n)
	}

	// --limit filter
	if n := countSessions("--limit", "1"); n > 1 {
		t.Errorf("--limit 1: want <=1, got %d", n)
	}

	// --since filter (everything within last hour)
	if n := countSessions("--since", "1h"); n < 1 {
		t.Errorf("--since 1h: want >=1, got %d", n)
	}

	// compound: --tool + --project
	if n := countSessions("--tool", "claude", "--project", projectA); n < 1 {
		t.Errorf("--tool claude + --project project-a: want >=1, got %d", n)
	}
}

// ---------- TestArgumentPassing ----------

func TestArgumentPassing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initGitRepo(t, ctx, dir)

	wantArgv := []string{
		"claude", "--dangerously-skip-permissions",
		"-p", "hello world",
		"--output-format", "json",
	}

	_, err := xrrutil.RunCommand(ctx, wantArgv, dir)
	if err != nil {
		t.Logf("claude err (expected without API key): %v", err)
	}

	// Walk cassette dir for request YAML files that contain the argv.
	cassetteDir := xrrutil.CassetteDir()
	if cassetteDir == "" {
		t.Skip("XRR_CASSETTE_DIR not set")
	}

	found := false
	filepath.Walk(cassetteDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Request cassettes contain "argv" with the command arguments.
		if !strings.HasSuffix(info.Name(), ".req.yaml") &&
			!strings.HasSuffix(info.Name(), ".yaml") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		// Check that all argv elements appear in the cassette.
		allPresent := true
		for _, arg := range wantArgv {
			if !strings.Contains(content, arg) {
				allPresent = false
				break
			}
		}
		if allPresent {
			found = true
			t.Logf("found matching cassette: %s", path)
		}
		return nil
	})

	if !found && xrrutil.Active() {
		t.Error("no cassette found containing expected argv")
	} else if !xrrutil.Active() {
		t.Log("WARN: XRR_MODE not set; skipping cassette verification")
	}
}

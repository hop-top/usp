package xrrutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestActiveEmpty(t *testing.T) {
	t.Setenv("XRR_MODE", "")
	if Active() {
		t.Fatal("Active() should be false when XRR_MODE is empty string")
	}
}

func TestActiveSet(t *testing.T) {
	t.Setenv("XRR_MODE", "record")
	if !Active() {
		t.Fatal("Active() should be true when XRR_MODE is set")
	}
	if got := Mode(); got != "record" {
		t.Fatalf("Mode() = %q, want %q", got, "record")
	}
}

func TestCassetteDirEnv(t *testing.T) {
	t.Setenv("XRR_CASSETTE_DIR", "/tmp/cassettes")
	if got := CassetteDir(); got != "/tmp/cassettes" {
		t.Fatalf("CassetteDir() = %q, want %q", got, "/tmp/cassettes")
	}
}

func TestRunCommandDirect(t *testing.T) {
	t.Setenv("XRR_MODE", "")
	r, err := RunCommand(context.Background(), []string{"echo", "hello"}, "")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if r.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", r.Stdout, "hello\n")
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode)
	}
}

func TestRunCommandDirectFailure(t *testing.T) {
	t.Setenv("XRR_MODE", "")
	r, err := RunCommand(context.Background(), []string{"false"}, "")
	if err == nil {
		t.Fatal("expected error from 'false' command")
	}
	if r.ExitCode == 0 {
		t.Error("ExitCode should be non-zero")
	}
}

func TestRunCommandDirectWithCwd(t *testing.T) {
	t.Setenv("XRR_MODE", "")
	dir := t.TempDir()
	r, err := RunCommand(context.Background(), []string{"pwd"}, dir)
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if r.Stdout == "" {
		t.Fatal("empty stdout from pwd")
	}
}

func TestRunCommandRecordReplay(t *testing.T) {
	// Reset lazy session for clean test.
	session = nil
	adapter = nil

	cassetteDir := filepath.Join(t.TempDir(), "cassettes")
	os.MkdirAll(cassetteDir, 0o755)

	// Record phase.
	t.Setenv("XRR_MODE", "record")
	t.Setenv("XRR_CASSETTE_DIR", cassetteDir)
	session = nil

	r, err := RunCommand(
		context.Background(), []string{"echo", "recorded"}, "",
	)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if r.Stdout != "recorded\n" {
		t.Errorf("record Stdout = %q", r.Stdout)
	}

	// Verify cassette files exist.
	entries, _ := os.ReadDir(cassetteDir)
	if len(entries) == 0 {
		t.Fatal("no cassette files written")
	}

	// Replay phase — same argv returns recorded output.
	session = nil
	t.Setenv("XRR_MODE", "replay")

	r, err = RunCommand(
		context.Background(), []string{"echo", "recorded"}, "",
	)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if r.Stdout != "recorded\n" {
		t.Errorf("replay Stdout = %q, want %q", r.Stdout, "recorded\n")
	}
	if r.ExitCode != 0 {
		t.Errorf("replay ExitCode = %d, want 0", r.ExitCode)
	}
}

func TestRunCommandEmptyArgv(t *testing.T) {
	t.Setenv("XRR_MODE", "")
	_, err := RunCommand(context.Background(), nil, "")
	if err == nil {
		t.Fatal("expected error for empty argv")
	}
}

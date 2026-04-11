// Package xrrutil wraps CLI exec calls through xrr when XRR_MODE
// is set. In normal mode, commands run directly. In record mode,
// argv + stdout + stderr are saved to cassettes. In replay mode,
// recorded output is returned without running the binary.
package xrrutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	xrr "hop.top/xrr"
	xrrexec "hop.top/xrr/adapters/exec"
)

// Active returns true when XRR_MODE is set.
func Active() bool { return os.Getenv("XRR_MODE") != "" }

// Mode returns the current XRR_MODE value.
func Mode() string { return os.Getenv("XRR_MODE") }

// CassetteDir returns XRR_CASSETTE_DIR.
func CassetteDir() string { return os.Getenv("XRR_CASSETTE_DIR") }

// session is lazily initialized on first RunCommand call.
var (
	session *xrr.FileSession
	adapter *xrrexec.Adapter
)

func ensureSession() {
	if session != nil {
		return
	}
	adapter = xrrexec.NewAdapter()
	dir := CassetteDir()
	if dir == "" {
		dir = os.TempDir()
	}
	os.MkdirAll(dir, 0o755)
	session = xrr.NewSession(
		xrr.Mode(Mode()),
		xrr.NewFileCassette(dir),
	)
}

// Result holds the output of a command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// RunCommand executes argv, routing through xrr when active.
//
// Normal mode: runs the command directly via os/exec.
// Record mode: runs + saves argv/stdout/stderr to cassette.
// Replay mode: returns recorded output without running.
func RunCommand(
	ctx context.Context, argv []string, cwd string,
) (*Result, error) {
	if !Active() {
		return runDirect(ctx, argv, cwd)
	}

	ensureSession()

	req := &xrrexec.Request{
		Argv: argv,
		Cwd:  cwd,
	}

	resp, err := session.Record(ctx, adapter, req,
		func() (xrr.Response, error) {
			r, execErr := runDirect(ctx, argv, cwd)
			if execErr != nil {
				// Return the result with exit code; the error
				// is captured in the cassette's error field.
				return &xrrexec.Response{
					Stdout:   r.Stdout,
					Stderr:   r.Stderr,
					ExitCode: r.ExitCode,
				}, execErr
			}
			return &xrrexec.Response{
				Stdout:   r.Stdout,
				Stderr:   r.Stderr,
				ExitCode: r.ExitCode,
			}, nil
		},
	)
	if err != nil && resp == nil {
		return nil, err
	}

	// Handle both typed and raw (replay) responses.
	switch v := resp.(type) {
	case *xrrexec.Response:
		return &Result{
			Stdout:   v.Stdout,
			Stderr:   v.Stderr,
			ExitCode: v.ExitCode,
		}, err
	case *xrr.RawResponse:
		return &Result{
			Stdout:   strval(v.Payload, "stdout"),
			Stderr:   strval(v.Payload, "stderr"),
			ExitCode: intval(v.Payload, "exit_code"),
		}, err
	default:
		return nil, fmt.Errorf(
			"xrrutil: unexpected response type %T", resp)
	}
}

func runDirect(
	ctx context.Context, argv []string, cwd string,
) (*Result, error) {
	if len(argv) == 0 {
		return &Result{ExitCode: 1}, fmt.Errorf("empty argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, err
}

func strval(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func intval(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

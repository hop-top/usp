// Package xrrutil provides helpers for xrr (cross-runtime recorder)
// integration. When XRR_MODE is set, CLI interactions can be recorded
// or replayed via xrr cassettes.
//
// Integration points that need xrr wrapping (Phase 3):
//   - uxp.Detect() — runs `<cli> --version` via os/exec
//   - resume command — runs syscall.Exec to hand off to target CLI
//   - adapter.GetSession / adapter.StreamTurns — read session data
package xrrutil

import "os"

// Active returns true when XRR_MODE is set (record, replay, or
// passthrough).
func Active() bool { return os.Getenv("XRR_MODE") != "" }

// Mode returns the current XRR_MODE value.
func Mode() string { return os.Getenv("XRR_MODE") }

// CassetteDir returns XRR_CASSETTE_DIR.
func CassetteDir() string { return os.Getenv("XRR_CASSETTE_DIR") }

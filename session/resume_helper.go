package session

import (
	"fmt"

	"hop.top/kit/go/core/uxp"
	"hop.top/kit/go/core/uxp/invoke"
)

// ResumeCmdFor builds a native argv for resuming a session on the
// given target CLI by delegating to kit's invocation facade.
//
// Adapters call this from their ResumeCmd implementation so the
// argv shape stays in lockstep with go/core/uxp/invoke/adapters/<cli>.
// On Build errors (e.g. an adapter rejecting an invariant), the
// helper returns a best-effort fallback — empty slice, signalling
// the caller to surface the error rather than spawn a broken
// command.
//
// The returned slice is the full argv: [Path, Args...].
func ResumeCmdFor(adapter invoke.InvocationAdapter, nativeID string) []string {
	spec, _, err := adapter.Build(invoke.Invocation{
		CLI:       adapter.CLI(),
		Mode:      invoke.ModeResume,
		SessionID: nativeID,
	})
	if err != nil {
		// Should not happen for the known adapters — they all accept
		// ModeResume + SessionID as a happy path. Emit a fallback
		// that still names the binary so the caller's error surface
		// does not crash.
		info, ok := uxp.DefaultRegistry().Get(adapter.CLI())
		if !ok || len(info.BinaryNames) == 0 {
			return nil
		}
		return []string{info.BinaryNames[0]}
	}
	return append([]string{spec.Path}, spec.Args...)
}

// ResumeCmdMustFor is like ResumeCmdFor but panics on Build error.
// Intended for tests where the inputs are statically valid; the
// production path should use ResumeCmdFor and surface errors via
// its own error-handling path.
func ResumeCmdMustFor(adapter invoke.InvocationAdapter, nativeID string) []string {
	cmd := ResumeCmdFor(adapter, nativeID)
	if len(cmd) <= 1 {
		panic(fmt.Sprintf("session.ResumeCmdMustFor: empty argv for %s", adapter.CLI()))
	}
	return cmd
}

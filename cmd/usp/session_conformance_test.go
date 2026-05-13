package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
)

// TestSessionSubtreeAnnotations asserts the per-leaf kit annotations
// that the strict CLI validator requires. Slice A owns the session
// subtree; siblings (top-level read/management leaves, resume, alias,
// status) land in slices B and C. A full root-level AssertCLI check
// has to wait until all three slices merge — until then this test
// pins down what slice A is responsible for.
func TestSessionSubtreeAnnotations(t *testing.T) {
	parent := sessionCmd(nil)

	type want struct {
		sideEffect    cli.SideEffect
		idempotency   cli.Idempotency
		requireLong   bool
		topLevelGroup bool
	}
	wants := map[string]want{
		"list":    {sideEffect: cli.SideEffectRead, idempotency: cli.IdempotencyYes, requireLong: true},
		"search":  {sideEffect: cli.SideEffectRead, idempotency: cli.IdempotencyYes, requireLong: true},
		"show":    {sideEffect: cli.SideEffectRead, idempotency: cli.IdempotencyYes, requireLong: true},
		"lineage": {sideEffect: cli.SideEffectRead, idempotency: cli.IdempotencyYes, requireLong: true},
		"skills":  {sideEffect: cli.SideEffectRead, idempotency: cli.IdempotencyYes, requireLong: true},
		"tools":   {sideEffect: cli.SideEffectRead, idempotency: cli.IdempotencyYes, requireLong: true},
	}

	seen := map[string]bool{}
	for _, sub := range parent.Commands() {
		name := sub.Name()
		w, ok := wants[name]
		if !ok {
			continue
		}
		seen[name] = true

		got, ok := cli.GetSideEffect(sub)
		if !ok {
			t.Errorf("session %s: missing kit/side-effect annotation", name)
		} else if got != w.sideEffect {
			t.Errorf("session %s: kit/side-effect = %q, want %q",
				name, got, w.sideEffect)
		}

		gotIdem, ok := cli.GetIdempotency(sub)
		if !ok {
			t.Errorf("session %s: missing kit/idempotent annotation", name)
		} else if gotIdem != w.idempotency {
			t.Errorf("session %s: kit/idempotent = %q, want %q",
				name, gotIdem, w.idempotency)
		}

		if w.requireLong {
			if strings.TrimSpace(sub.Long) == "" {
				t.Errorf("session %s: Long help is empty", name)
			}
			// Sanity: Long must not be a one-line repeat of Short.
			if strings.TrimSpace(sub.Long) == strings.TrimSpace(sub.Short) {
				t.Errorf("session %s: Long help duplicates Short", name)
			}
		}

		// Depth-2 leaves under a noun group: must NOT carry
		// kit/top-level-verb. That annotation is reserved for the
		// depth-1 leaves owned by slice B/C.
		if cli.IsTopLevelVerb(sub) {
			t.Errorf("session %s: depth-2 leaf must not declare kit/top-level-verb",
				name)
		}
	}

	for name := range wants {
		if !seen[name] {
			t.Errorf("session subtree: expected leaf %q not registered on session group", name)
		}
	}
}

// TestSessionGroupShape pins down the session noun-group node itself.
// At depth-1 with subcommands, kit treats it as a group, not a
// runnable leaf — so it does not need kit/side-effect or
// kit/idempotent, and Long is not required by the validator. Short
// must be set (group nodes are checked for Short).
func TestSessionGroupShape(t *testing.T) {
	parent := sessionCmd(nil)
	if parent.Runnable() {
		t.Errorf("session: expected non-runnable group node, got runnable")
	}
	if !parent.HasSubCommands() {
		t.Errorf("session: expected subcommands attached")
	}
	if strings.TrimSpace(parent.Short) == "" {
		t.Errorf("session: Short help is empty")
	}
}

// Compile-time hint: the assertions above use cobra.Command directly,
// so the import is intentional even when no field is referenced.
var _ = (*cobra.Command)(nil)

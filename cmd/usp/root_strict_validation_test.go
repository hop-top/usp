// Package main — root_strict_validation_test.go pins the kit
// 12fcc-leak strict validation gate against usp's production root.
//
// Background: kit/go/console/cli's Layer-A validator (Root.Validate)
// walks every runnable leaf and demands that each one carry a
// kit/side-effect annotation (and a kit/idempotent annotation, with
// a default fill for reads). It also enforces the top-level shape:
// `status` subcommand reserved, Short/Long required, no shadow of
// global flags, etc. Phases 1-3 of refactor/kit-12fcc-integration
// hand-annotated every usp leaf so this gate passes. This test
// guards against a future regression where someone adds a new leaf
// without an annotation — that drop must fail the suite, not slip
// into a binary that aborts at first `Execute`.
//
// We deliberately build the SAME root that cmd/usp/main.go ships:
// the production statusOption, the production subcommand tree, the
// production alias mount. DisableValidate stays false (the strict
// gate is on by default; we just need it to come back clean).
package main

import (
	"path/filepath"
	"testing"

	"hop.top/kit/go/console/alias"
	"hop.top/kit/go/console/cli"
)

// buildProductionRoot mirrors main()'s wiring without the Execute()
// call. Anything that would block validation (missing annotation,
// reserved-name collision, top-level leaf without a side-effect)
// surfaces from root.Validate().
//
// Note: we do NOT pass DisableValidate. The whole point of this
// test is to exercise the strict gate exactly as the shipped
// binary does on boot.
func buildProductionRoot(t *testing.T) *cli.Root {
	t.Helper()
	root := cli.New(
		cli.Config{
			Name:          "usp",
			Version:       "test",
			Short:         "Universal Sessions Protocol — cross-CLI session management",
			Accent:        "#7C5CFF",
			Help:          cli.HelpConfig{Groups: rootGroups()},
			ProjectMarker: ".usp.yaml",
		},
		statusOption(),
	)
	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		mcpCmd(),
		versionCmd(),
		configCmd(root.Viper),
		upgradeCmd(),
	)
	// Alias subtree: kit-owned subcommand registered against an
	// in-memory store (no real $XDG_CONFIG_HOME write).
	store := alias.NewStore(filepath.Join(t.TempDir(), "aliases.yaml"))
	root.Cmd.AddCommand(aliasCmd(root, store))
	applyCommandGroups(root.Cmd)
	registerHints(root)
	return root
}

// TestRootValidate_StrictGatePasses asserts that the production
// root passes kit's strict Layer-A validation. If a future commit
// lands a new leaf without `cli.SetSideEffect` / `cli.SetIdempotent`,
// or reuses a kit-reserved name (`status`, `completion`, `help`),
// this test fails before the binary is shipped.
//
// Symptoms that point here:
//
//   - `usp --help` exits non-zero with a "kit/side-effect missing"
//     error during cli.Execute's pre-flight.
//   - cmd path regressions like a renamed group losing its annotation
//     propagation.
//
// Fix path: annotate the new leaf via cli.SetSideEffect (read,
// write-local, write-shared, interactive, or destructive*) and
// cli.SetIdempotent (true/false). See docs/migration/kit-12fcc-confirm.md
// for the side-effect ladder and audit table.
func TestRootValidate_StrictGatePasses(t *testing.T) {
	root := buildProductionRoot(t)

	if err := root.Validate(); err != nil {
		t.Fatalf("strict Root.Validate failed on production tree: %v\n"+
			"Every runnable leaf must carry a kit/side-effect annotation. "+
			"See docs/migration/kit-12fcc-confirm.md for the ladder.", err)
	}
}

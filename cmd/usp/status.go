// Package main — status.go wires kit's reserved `<tool> status`
// subcommand onto usp's root.
//
// usp inherits the full kit-shipped status surface (six default
// providers: profile, env, workspace, auth, effective-config,
// kit-annotations). Kit's WithStatus self-annotates the leaf for §4
// (Layer-A) conformance — kit/side-effect=read, kit/idempotent=yes,
// kit/top-level-verb — and seeds reservedSnapshot so the validator
// stops flagging `usp` for the missing-reserved-status bucket.
//
// Quoting kit's contract at go/console/cli/status.go:113-130:
//
//	WithStatus returns a cli.New option that mounts the kit-shipped
//	`<tool> status` subcommand and seeds the reservedSnapshot. The
//	subcommand walks every registered StatusProvider (defaults +
//	adopter-registered) and renders a StatusOutput in the active
//	--format.
//
// usp-specific providers (index DB readiness, configured CLI
// adapters, build/binary metadata) can be layered on later via
// root.RegisterStatusProvider(...) once the slice integrator wires a
// status hook into the API service. The baseline (T-0120) only
// requires presence of the reserved subcommand; provider extension
// is a follow-up.
package main

import (
	"hop.top/kit/go/console/cli"
)

// statusOption returns the cli.New functional option that mounts
// `usp status`. Kept in its own file so the slice diff is reviewable
// in isolation; called from main.go inside the cli.New(...) call.
func statusOption() func(*cli.Root) {
	return cli.WithStatus(cli.StatusConfig{
		// ExtraEnvKeys: surface USP_* alongside KIT_* in the env section.
		ExtraEnvKeys: []string{"USP_*"},
	})
}

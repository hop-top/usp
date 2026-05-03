// Package main — alias.go wires the kit primitive
// hop.top/kit/go/console/alias (a YAML-backed Store) into usp's cobra
// tree under the MANAGEMENT group.
//
// Storage: $XDG_CONFIG_HOME/usp/aliases.yaml. usp is single-tenant
// local-only; there is no project-scoped overlay (cf. tlc).
//
// Surfaces:
//   - `usp alias` / `alias list`        — dump active aliases
//   - `usp alias add <name> <target...>` — set/update an alias
//   - `usp alias remove <name>`          — drop an alias
//
// Stored aliases are also registered as runtime command shims so
// `usp <alias>` dispatches to the target subcommand.
package main

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"hop.top/kit/go/console/alias"
	"hop.top/kit/go/console/cli"
	"hop.top/kit/go/core/xdg"
)

// aliasStorePath returns the YAML path for usp aliases. Errors from
// xdg propagate to the caller so wiring can fall back gracefully.
func aliasStorePath() (string, error) {
	dir, err := xdg.ConfigDir("usp")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "aliases.yaml"), nil
}

// loadAliasStore returns a Store with its YAML loaded (missing file
// is not an error — the store stays empty).
func loadAliasStore() (*alias.Store, error) {
	path, err := aliasStorePath()
	if err != nil {
		return nil, err
	}
	s := alias.NewStore(path)
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

// aliasCmd builds the MANAGEMENT-grouped `alias` subtree backed by
// the kit YAML store.
func aliasCmd(root *cli.Root, store *alias.Store) *cobra.Command {
	return root.AliasCmd(store)
}

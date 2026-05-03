package main

import (
	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
)

// commandGroups maps subcommand names to a group id. New commands
// should be registered here, not via per-cmd GroupID literals.
var commandGroups = map[string]string{
	"session": "knowledge",
	"resume":  "lifecycle",
	"doctor":  "organize",
	"setup":   "organize",
	"version": "management",
}

// rootGroups returns the custom group definitions in display order.
// kit/cli auto-registers MANAGEMENT (hidden); listing it again would
// duplicate it in --help-all output.
func rootGroups() []cli.GroupConfig {
	return []cli.GroupConfig{
		{ID: "knowledge", Title: "KNOWLEDGE"},
		{ID: "lifecycle", Title: "LIFECYCLE"},
		{ID: "organize", Title: "ORGANIZE"},
	}
}

// applyCommandGroups walks the root's direct subcommands and assigns
// each a GroupID from commandGroups when one matches.
func applyCommandGroups(root *cobra.Command) {
	for _, c := range root.Commands() {
		if g, ok := commandGroups[c.Name()]; ok {
			c.GroupID = g
		}
	}
}

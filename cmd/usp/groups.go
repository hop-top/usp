package main

import "github.com/spf13/cobra"

// commandGroups maps subcommand names to a group id. New commands
// should be registered here, not via per-cmd GroupID literals.
var commandGroups = map[string]string{
	"session": "knowledge",
	"resume":  "lifecycle",
	"doctor":  "organize",
	"install": "organize",
	"setup":   "organize",
}

// rootGroups returns the cobra group definitions in display order.
// kit/cli v0.3.2-patch.3 has no Help/Groups field on Config; groups
// are wired via cobra.Command.AddGroup directly.
func rootGroups() []*cobra.Group {
	return []*cobra.Group{
		{ID: "knowledge", Title: "KNOWLEDGE"},
		{ID: "lifecycle", Title: "LIFECYCLE"},
		{ID: "organize", Title: "ORGANIZE"},
		{ID: "management", Title: "MANAGEMENT"},
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

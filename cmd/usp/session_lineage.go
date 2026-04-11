package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"hop.top/usp/lineage"
	"hop.top/usp/session"
)

func sessionLineageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lineage <id>",
		Short: "Show cross-CLI session lineage",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id := args[0]

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("home dir: %w", err)
			}
			dbPath := filepath.Join(home, ".local", "state", "usp", "sessions.db")

			store, err := lineage.Open(dbPath)
			if err != nil {
				return tryNativeLineage(id)
			}
			defer store.Close()

			sess, err := store.GetSession(id)
			if err != nil {
				return tryNativeLineage(id)
			}

			printLineage(sess)
			return nil
		},
	}
}

// tryNativeLineage falls back to native adapters for a single-segment display.
func tryNativeLineage(id string) error {
	adapters := allAdapters()
	for _, name := range adapterOrder(id) {
		a, ok := adapters[name]
		if !ok {
			continue
		}
		s, err := a.GetSession(id)
		if err != nil || s == nil {
			continue
		}
		s.Segments = []session.Segment{{
			CLI:       s.CLI,
			NativeID:  s.ID,
			StartedAt: s.StartedAt,
			TurnCount: s.TurnCount,
		}}
		printLineage(s)
		return nil
	}
	return fmt.Errorf("session %q not found", id)
}

func printLineage(sess *session.Session) {
	w := os.Stdout
	fmt.Fprintf(w, "Session: %s\nProject: %s\n\n", sess.ID, sess.ProjectCwd)

	fmt.Fprintf(w, "  %-3s %-10s %-14s %-21s %s\n",
		"#", "CLI", "Native ID", "Started", "Turns")

	var totalTurns int
	clis := map[string]struct{}{}
	for i, seg := range sess.Segments {
		clis[string(seg.CLI)] = struct{}{}
		totalTurns += seg.TurnCount
		fmt.Fprintf(w, "  %-3d %-10s %-14s %-21s %5d\n",
			i+1,
			seg.CLI,
			truncateID(seg.NativeID, 12),
			seg.StartedAt.Format("2006-01-02 15:04:05"),
			seg.TurnCount,
		)
	}

	noun := "CLI"
	if len(clis) != 1 {
		noun = "CLIs"
	}
	fmt.Fprintf(w, "\nTotal: %d turns across %d %s\n",
		totalTurns, len(clis), noun)
}

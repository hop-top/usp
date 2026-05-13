package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"hop.top/kit/go/console/cli"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/lineage"
	"hop.top/usp/session"
)

func sessionLineageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lineage <id>",
		Short: "Show cross-CLI session lineage",
		Long: `Show the cross-CLI lineage of a single session as an ordered
list of segments.

The positional <id> accepts either the canonical TypeID or the
native CLI session id. The lineage store is consulted first; on a
miss the command falls back to the native adapter resolver and
displays a one-segment lineage for the single CLI that owns the
id.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			input := args[0]

			dbPath, err := lineageDBPath()
			if err != nil {
				return fmt.Errorf("lineage path: %w", err)
			}

			store, err := lineage.Open(dbPath)
			if err != nil {
				return tryNativeLineage(input)
			}
			defer store.Close()

			// Lineage store keys by native id today; if user passed
			// a TypeID, fall through to the native adapter resolver
			// which knows both forms.
			sess, err := store.GetSession(input)
			if err != nil {
				return tryNativeLineage(input)
			}

			printLineage(sess)
			return nil
		},
	}
	cli.SetSideEffect(cmd, cli.SideEffectRead)
	cli.SetIdempotency(cmd, cli.IdempotencyYes)
	return cmd
}

// tryNativeLineage falls back to native adapters for a single-segment display.
func tryNativeLineage(id string) error {
	adapters := allAdapters()
	s, _, _, err := sessionutil.ResolveSessionID(
		id, adapters, adapterOrder(id))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return exitNotFound(err)
		}
		return err
	}
	s.Segments = []session.Segment{{
		CLI:       s.CLI,
		NativeID:  s.NativeID,
		StartedAt: s.StartedAt,
		TurnCount: s.TurnCount,
	}}
	printLineage(s)
	return nil
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

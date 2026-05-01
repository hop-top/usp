package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"hop.top/kit/uxp"
	"hop.top/usp/index"
)

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install [cli|all]",
		Short: "Detect CLIs and index sessions",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "all"
			if len(args) > 0 {
				target = args[0]
			}

			reg := uxp.DefaultRegistry()
			adapters := allAdapters()

			names := reg.Names()
			if target != "all" {
				if _, ok := reg.Get(target); !ok {
					return fmt.Errorf("unknown CLI %q", target)
				}
				names = []uxp.CLIName{target}
			}

			// Open/create project index.
			idxPath, err := indexDBPath()
			if err != nil {
				return fmt.Errorf("resolve index path: %w", err)
			}
			if err := os.MkdirAll(filepath.Dir(idxPath), 0o750); err != nil {
				return fmt.Errorf("create index dir: %w", err)
			}

			idx, err := index.Open(idxPath)
			if err != nil {
				return fmt.Errorf("open index: %w", err)
			}
			defer idx.Close()

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Installing USP adapters...")
			fmt.Fprintln(w)

			var detected, totalSessions int

			for _, name := range names {
				res, err := uxp.Detect(name, reg, nil)
				if err != nil {
					fmt.Fprintf(w, "  %-14s -         %s  %s\n",
						name, statusSyms[2], "error: "+err.Error())
					continue
				}

				if !res.Installed {
					fmt.Fprintf(w, "  %-14s -         %s  not found\n",
						name, statusSyms[2])
					continue
				}

				detected++
				ver := res.Version
				if ver == "" {
					ver = "?"
				}

				// Count sessions if we have an adapter.
				var count int
				if a, ok := adapters[name]; ok {
					sessions, listErr := a.ListSessions("")
					if listErr == nil {
						count = len(sessions)
					}
				}
				totalSessions += count

				suffix := fmt.Sprintf("%d sessions", count)
				if count == 0 {
					suffix = "0 sessions (no transcripts)"
				}

				fmt.Fprintf(w, "  %-14s v%-14s %s  detected    %s\n",
					name, ver, statusSyms[0], suffix)
			}

			fmt.Fprintln(w)
			fmt.Fprintf(w, "Summary: %d/%d CLIs detected, %d sessions indexed\n",
				detected, len(names), totalSessions)

			fmt.Fprintf(w, "Index: %s\n", idxPath)

			return nil
		},
	}
}

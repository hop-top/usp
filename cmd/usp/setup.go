package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"hop.top/kit/go/console/output"
	"hop.top/kit/go/core/uxp"
	"hop.top/usp/index"
)

// setupRow is the table-renderable row for the setup summary.
type setupRow struct {
	CLI      string `table:"CLI"      json:"cli"               yaml:"cli"`
	Version  string `table:"VERSION"  json:"version"           yaml:"version"`
	Status   string `table:"STATUS"   json:"status"            yaml:"status"`
	Sessions string `table:"SESSIONS" json:"sessions"          yaml:"sessions"`
	Error    string `table:"-"        json:"error,omitempty"   yaml:"error,omitempty"`
}

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup [cli]",
		Short: "Detect CLIs and index sessions",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if err := runSetup(c.Context(), args); err != nil {
				return err
			}
			emitHint("setup")
			return nil
		},
	}
}

func runSetup(ctx context.Context, args []string) error {
	target := ""
	if len(args) > 0 {
		target = args[0]
	}

	reg := uxp.DefaultRegistry()
	adapters := allAdapters()

	names := reg.Names()
	if target != "" {
		if _, ok := reg.Get(target); !ok {
			return fmt.Errorf("unknown CLI %q", target)
		}
		names = []uxp.CLIName{target}
	}

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
	defer func() { _ = idx.Close() }()

	rows := make([]setupRow, 0, len(names))

	if err := runWithProgress(ctx, "setup", "detecting CLIs and loading sessions", func() error {
		for _, name := range names {
			res, dErr := uxp.Detect(name, reg, nil)
			if dErr != nil {
				rows = append(rows, setupRow{
					CLI:    string(name),
					Status: statusSym[uxp.StatusFail],
					Error:  dErr.Error(),
				})
				continue
			}
			if !res.Installed {
				rows = append(rows, setupRow{
					CLI:    string(name),
					Status: statusSym[uxp.StatusFail],
					Error:  "not found",
				})
				continue
			}
			ver := res.Version
			if ver == "" {
				ver = "?"
			}
			var count int
			if a, ok := adapters[name]; ok {
				if sessions, lErr := a.ListSessions(""); lErr == nil {
					count = len(sessions)
				}
			}
			suffix := fmt.Sprintf("%d", count)
			if count == 0 {
				suffix = "0 (no transcripts)"
			}
			rows = append(rows, setupRow{
				CLI:      string(name),
				Version:  "v" + ver,
				Status:   statusSym[uxp.StatusOK],
				Sessions: suffix,
			})
		}
		return nil
	}); err != nil {
		return err
	}

	if err := output.Render(os.Stdout, formatFromViper(), rows); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if formatFromViper() == output.Table {
		fmt.Fprintf(os.Stdout, "\nIndex: %s\n", idxPath)
	}
	return nil
}

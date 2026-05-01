package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"hop.top/kit/output"
	"hop.top/kit/uxp"
	"hop.top/usp/index"
)

// installRow is the table-renderable row for the setup/install summary.
type installRow struct {
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
		RunE: func(_ *cobra.Command, args []string) error {
			return runSetup(args)
		},
	}
}

// installCmd is the deprecated alias for setupCmd.
func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "install [cli]",
		Short:  "(deprecated) Use `usp setup` instead",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			slog.Warn("install is deprecated; use `setup` instead")
			return runSetup(args)
		},
	}
}

// runSetup is the shared body for install/setup commands.
func runSetup(args []string) error {
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
	defer idx.Close()

	rows := make([]installRow, 0, len(names))

	for _, name := range names {
		res, dErr := uxp.Detect(name, reg, nil)
		if dErr != nil {
			rows = append(rows, installRow{
				CLI:    string(name),
				Status: statusSym[uxp.StatusFail],
				Error:  dErr.Error(),
			})
			continue
		}
		if !res.Installed {
			rows = append(rows, installRow{
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
		rows = append(rows, installRow{
			CLI:      string(name),
			Version:  "v" + ver,
			Status:   statusSym[uxp.StatusOK],
			Sessions: suffix,
		})
	}

	if err := output.Render(os.Stdout, formatFromViper(), rows); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if formatFromViper() == output.Table {
		fmt.Fprintf(os.Stdout, "\nIndex: %s\n", idxPath)
	}
	return nil
}

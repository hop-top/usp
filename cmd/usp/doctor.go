package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"hop.top/kit/go/console/output"
	"hop.top/kit/go/core/uxp"
)

// statusSym maps a uxp CheckStatus to a stable symbol. Plain glyphs;
// fang/lipgloss colors at render time when format=table on a tty.
var statusSym = map[uxp.CheckStatus]string{
	uxp.StatusOK:   "✓",
	uxp.StatusWarn: "⚠",
	uxp.StatusFail: "✗",
	uxp.StatusSkip: "—",
}

// doctorRow is the table-renderable projection of a uxp.Check.
type doctorRow struct {
	Name    string `table:"CHECK"   json:"name"             yaml:"name"`
	Status  string `table:"STATUS"  json:"status"           yaml:"status"`
	Message string `table:"MESSAGE" json:"message"          yaml:"message"`
	Detail  string `table:"DETAIL"  json:"detail,omitempty" yaml:"detail,omitempty"`
}

func doctorCmd() *cobra.Command {
	var cliFlag string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check environment health for supported CLIs",
		RunE: func(_ *cobra.Command, _ []string) error {
			reg := uxp.DefaultRegistry()
			doc := uxp.NewDoctor()

			names := reg.Names()
			if cliFlag != "" {
				if _, ok := reg.Get(cliFlag); !ok {
					return fmt.Errorf("unknown CLI %q", cliFlag)
				}
				names = []uxp.CLIName{cliFlag}
			}

			for _, name := range names {
				doc.Add(uxp.CheckCLIInstalled(name, nil))
				doc.Add(uxp.CheckStoreExists(name, reg))
				doc.Add(checkStoreReadable(name, reg))
			}
			doc.Add(checkProjectIndex())

			results := doc.Run()
			doctorHadFailure = false
			for _, c := range results {
				if c.Status == uxp.StatusFail {
					doctorHadFailure = true
					break
				}
			}
			if err := output.Render(os.Stdout, formatFromViper(), toDoctorRows(results)); err != nil {
				return err
			}
			emitHint("doctor")
			return nil
		},
	}

	cmd.Flags().StringVar(&cliFlag, "tool", "",
		"Check a single CLI (e.g. claude, codex)")
	return cmd
}

// formatFromViper reads the persistent --format global from the active
// rootViper; defaults to table.
func formatFromViper() output.Format {
	f := rootViper.GetString("format")
	if f == "" {
		return output.Table
	}
	return f
}

func toDoctorRows(checks []uxp.Check) []doctorRow {
	rows := make([]doctorRow, len(checks))
	for i, c := range checks {
		sym, ok := statusSym[c.Status]
		if !ok {
			sym = "?"
		}
		rows[i] = doctorRow{
			Name:    string(c.Name),
			Status:  sym,
			Message: c.Message,
			Detail:  c.Detail,
		}
	}
	return rows
}

// checkStoreReadable returns a CheckFunc that verifies the store dir
// is readable (can be listed).
func checkStoreReadable(cli uxp.CLIName, reg *uxp.CLIRegistry) uxp.CheckFunc {
	return func() uxp.Check {
		label := cli + "-store-readable"
		p, err := uxp.ResolveStorePath(cli, reg)
		if err != nil {
			return uxp.Check{
				Name: label, Status: uxp.StatusSkip,
				Message: "store path unknown",
			}
		}

		f, err := os.Open(p)
		if err != nil {
			if os.IsNotExist(err) {
				return uxp.Check{
					Name: label, Status: uxp.StatusSkip,
					Message: "store not present",
				}
			}
			return uxp.Check{
				Name: label, Status: uxp.StatusFail,
				Message: "store not readable",
				Detail:  err.Error(),
			}
		}
		_ = f.Close()

		return uxp.Check{
			Name: label, Status: uxp.StatusOK,
			Message: "store readable",
			Detail:  p,
		}
	}
}

// checkProjectIndex returns a CheckFunc that verifies the usp
// project index DB exists and is accessible.
func checkProjectIndex() uxp.CheckFunc {
	return func() uxp.Check {
		label := "project-index"
		p, err := indexDBPath()
		if err != nil {
			return uxp.Check{
				Name: label, Status: uxp.StatusWarn,
				Message: "cannot resolve data dir",
				Detail:  err.Error(),
			}
		}

		fi, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return uxp.Check{
					Name: label, Status: uxp.StatusWarn,
					Message: "index DB not found (run usp scan first?)",
					Detail:  p,
				}
			}
			return uxp.Check{
				Name: label, Status: uxp.StatusFail,
				Message: "index DB inaccessible",
				Detail:  err.Error(),
			}
		}

		if fi.IsDir() {
			return uxp.Check{
				Name: label, Status: uxp.StatusFail,
				Message: "index path is a directory, expected file",
				Detail:  p,
			}
		}

		return uxp.Check{
			Name: label, Status: uxp.StatusOK,
			Message: "index DB exists",
			Detail:  p,
		}
	}
}

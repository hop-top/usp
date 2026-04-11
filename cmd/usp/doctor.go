package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"hop.top/kit/uxp"
)

var statusSyms = [4]string{
	"\033[32m✓\033[0m", // OK
	"\033[33m⚠\033[0m", // Warn
	"\033[31m✗\033[0m", // Fail
	"\033[90m—\033[0m", // Skip
}

// defaultIndexPath returns the conventional project index DB path.
func defaultIndexPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "usp", "index.db")
}

func doctorCmd() *cobra.Command {
	var cliFlag string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check environment health for supported CLIs",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
				// Built-in: binary installed?
				doc.Add(uxp.CheckCLIInstalled(name, nil))

				// Built-in: store exists?
				doc.Add(uxp.CheckStoreExists(name, reg))

				// USP-specific: store readable?
				doc.Add(checkStoreReadable(name, reg))
			}

			// USP-specific: project index DB accessible.
			doc.Add(checkProjectIndex())

			results := doc.Run()
			printTable(cmd, results)
			return nil
		},
	}

	cmd.Flags().StringVar(&cliFlag, "tool", "",
		"Check a single CLI (e.g. claude, codex)")
	return cmd
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
				Detail: err.Error(),
			}
		}
		_ = f.Close()

		return uxp.Check{
			Name: label, Status: uxp.StatusOK,
			Message: "store readable",
			Detail: p,
		}
	}
}

// checkProjectIndex returns a CheckFunc that verifies the usp
// project index DB exists and is accessible.
func checkProjectIndex() uxp.CheckFunc {
	return func() uxp.Check {
		label := "project-index"
		p := defaultIndexPath()
		if p == "" {
			return uxp.Check{
				Name: label, Status: uxp.StatusWarn,
				Message: "cannot determine home directory",
			}
		}

		fi, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return uxp.Check{
					Name: label, Status: uxp.StatusWarn,
					Message: "index DB not found (run usp scan first?)",
					Detail: p,
				}
			}
			return uxp.Check{
				Name: label, Status: uxp.StatusFail,
				Message: "index DB inaccessible",
				Detail: err.Error(),
			}
		}

		if fi.IsDir() {
			return uxp.Check{
				Name: label, Status: uxp.StatusFail,
				Message: "index path is a directory, expected file",
				Detail: p,
			}
		}

		return uxp.Check{
			Name: label, Status: uxp.StatusOK,
			Message: "index DB exists",
			Detail: p,
		}
	}
}

// printTable renders doctor results as an aligned table.
func printTable(cmd *cobra.Command, checks []uxp.Check) {
	if len(checks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No checks configured.")
		return
	}

	// Compute column widths.
	nameW, msgW := 5, 7 // min: "Check", "Message"
	for _, c := range checks {
		if len(c.Name) > nameW {
			nameW = len(c.Name)
		}
		if len(c.Message) > msgW {
			msgW = len(c.Message)
		}
	}

	w := cmd.OutOrStdout()

	// Header.
	fmt.Fprintf(w, "  %-*s  %s  %-*s\n",
		nameW, "Check", "Status", msgW, "Message")
	fmt.Fprintf(w, "  %s  %s  %s\n",
		strings.Repeat("─", nameW),
		strings.Repeat("─", 6),
		strings.Repeat("─", msgW))

	for _, c := range checks {
		fmt.Fprintf(w, "  %-*s  %-9s %-*s\n",
			nameW, c.Name,
			statusSyms[c.Status],
			msgW, c.Message)
	}
}

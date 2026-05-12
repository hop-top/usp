package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"hop.top/kit/go/console/cli"
	kitlog "hop.top/kit/go/console/log"
	"hop.top/usp/internal/xrrutil"
)

// exitErr wraps an error with a custom process exit code per spec §8.1.
//
//	0 success    1 generic     2 usage (cobra)
//	3 not found  4 exists      5 unauthorized
type exitErr struct {
	code int
	err  error
}

func (e *exitErr) Error() string { return e.err.Error() }
func (e *exitErr) Unwrap() error { return e.err }

// exitNotFound returns an exitErr with code 3.
func exitNotFound(err error) error {
	return &exitErr{code: 3, err: err}
}

var version = "dev"

// rootViper is the active viper instance bound by cli.New. Subcommand
// RunE bodies read globals (format, quiet, no-color, no-hints) from
// here. Set in main; defaults to viper.New() for unit tests.
var rootViper = viper.New()
var activeRoot *cli.Root

func main() {
	var root *cli.Root
	root = cli.New(cli.Config{
		Name:          "usp",
		Version:       version,
		Short:         "Universal Sessions Protocol — cross-CLI session management",
		Accent:        "#7C5CFF",
		Help:          cli.HelpConfig{Groups: rootGroups()},
		ProjectMarker: ".usp.yaml",
		// Layer-A annotations + reserved `status` subcommand not yet
		// adopted; tracked separately. Re-enable when compliant.
		DisableValidate: true,
		Hooks: cli.Hooks{
			// -V/--verbose count → slog level (Info → Debug → Trace).
			// Re-init after flag parse; kit fills VerboseCount via OnInitialize.
			PrePersistentRunE: func(_ *cobra.Command, _ []string) error {
				paths, overrides, err := root.ConfigArgs()
				if err != nil {
					return err
				}
				if _, err := loadConfigWithLayers(
					root.Viper, paths, overrides,
				); err != nil {
					return fmt.Errorf("config: %w", err)
				}
				slog.SetDefault(slog.New(
					kitlog.WithVerbose(root.Viper, root.VerboseCount())))
				return nil
			},
		},
	},
		statusOption(),
	)
	rootViper = root.Viper
	activeRoot = root

	// Pre-parse default at Info level. Replaced by hook after parse.
	slog.SetDefault(slog.New(kitlog.New(root.Viper)))

	if xrrutil.Active() {
		fmt.Fprintf(root.Cmd.ErrOrStderr(),
			"xrr: mode=%s cassette_dir=%s\n",
			xrrutil.Mode(), xrrutil.CassetteDir())
	}

	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
		mcpCmd(),
		versionCmd(),
		configCmd(root.Viper),
		upgradeCmd(),
	)

	// Aliases: register the management subcommand + load stored
	// entries as runtime shims. Failures are logged and ignored so a
	// corrupt aliases.yaml never blocks the rest of the CLI.
	if store, err := loadAliasStore(); err == nil {
		root.Cmd.AddCommand(aliasCmd(root, store))
		if err := root.LoadAliasStore(store); err != nil {
			fmt.Fprintf(root.Cmd.ErrOrStderr(), "alias: %v\n", err)
		}
	} else {
		fmt.Fprintf(root.Cmd.ErrOrStderr(), "alias: %v\n", err)
	}

	applyCommandGroups(root.Cmd)
	registerHints(root)

	if err := root.Execute(context.Background()); err != nil {
		var ee *exitErr
		if errors.As(err, &ee) {
			os.Exit(ee.code)
		}
		os.Exit(1)
	}
}

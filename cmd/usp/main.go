package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
	"hop.top/kit/cli"
	kitlog "hop.top/kit/log"
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

func main() {
	root := cli.New(cli.Config{
		Name:    "usp",
		Version: version,
		Short:   "Universal Sessions Protocol — cross-CLI session management",
		Accent:  "#7C5CFF",
	})
	rootViper = root.Viper

	// Default slog handler: charm log via kit/log (stderr, viper-aware
	// "quiet"/"no-color"). Logger implements slog.Handler.
	slog.SetDefault(slog.New(kitlog.New(root.Viper)))

	registerConfigGlobals(root.Cmd, root.Viper)
	if _, err := loadConfig(root.Viper); err != nil {
		fmt.Fprintf(root.Cmd.ErrOrStderr(), "config: %v\n", err)
	}

	if xrrutil.Active() {
		fmt.Fprintf(root.Cmd.ErrOrStderr(),
			"xrr: mode=%s cassette_dir=%s\n",
			xrrutil.Mode(), xrrutil.CassetteDir())
	}

	for _, g := range rootGroups() {
		root.Cmd.AddGroup(g)
	}

	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		setupCmd(),
	)
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

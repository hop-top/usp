package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
	kitlog "hop.top/kit/log"
	"hop.top/kit/cli"
	"hop.top/usp/internal/xrrutil"
)

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

	if xrrutil.Active() {
		fmt.Fprintf(root.Cmd.ErrOrStderr(),
			"xrr: mode=%s cassette_dir=%s\n",
			xrrutil.Mode(), xrrutil.CassetteDir())
	}

	root.Cmd.AddCommand(
		sessionCmd(root),
		resumeCmd(),
		doctorCmd(),
		installCmd(),
	)

	if err := root.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}

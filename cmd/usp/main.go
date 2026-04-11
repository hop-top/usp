package main

import (
	"context"
	"fmt"
	"os"

	"hop.top/kit/cli"
	"hop.top/usp/internal/xrrutil"
)

var version = "dev"

func main() {
	root := cli.New(cli.Config{
		Name:    "usp",
		Version: version,
		Short:   "Universal Sessions Protocol — cross-CLI session management",
		Accent:  "#7C5CFF",
	})

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

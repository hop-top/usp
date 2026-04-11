package main

import (
	"context"
	"os"

	"hop.top/kit/cli"
)

var version = "dev"

func main() {
	root := cli.New(cli.Config{
		Name:    "usp",
		Version: version,
		Short:   "Universal Sessions Protocol — cross-CLI session management",
		Accent:  "#7C5CFF",
	})

	root.Cmd.AddCommand(
		sessionCmd(root),
		doctorCmd(),
		installCmd(),
	)

	if err := root.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}

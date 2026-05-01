package main

import (
	"image/color"
	"os"

	"hop.top/kit/cli"
	"hop.top/kit/output"
)

// hintMuted is the color used for next-step hints. Plain gray to match
// other hop-top tools without depending on the kit theme.
var hintMuted = color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}

// rootHints is set by registerHints; subcommand RunE bodies look up
// their hints here and pass them to output.RenderHints after primary
// output. nil-safe (test paths skip without panic).
var rootHints *output.HintSet

// registerHints attaches per-command next-step suggestions on root.Hints.
// Conditional hints toggle package vars below from inside RunE; static
// hints fire whenever the cmd completes successfully.
func registerHints(root *cli.Root) {
	rootHints = root.Hints
	root.Hints.Register("setup", output.Hint{
		Message: "Try `usp session list` to view indexed sessions.",
	})
	root.Hints.Register("doctor", output.Hint{
		Message:   "Run `usp setup <cli>` to fix.",
		Condition: func() bool { return doctorHadFailure },
	})
	root.Hints.Register("resume", output.Hint{
		Message:   "Run `usp session lineage <id>` to see the chain.",
		Condition: func() bool { return resumeLastUSPID != "" },
	})
	root.Hints.Register("list", output.Hint{
		Message:   "Run `usp setup` first.",
		Condition: func() bool { return listEmptyResult },
	})
}

// emitHint renders any hints registered for cmdName. RenderHints
// gates on tty + format=table + viper "no-hints"/"quiet"; cheap when
// disabled.
func emitHint(cmdName string) {
	if rootHints == nil || rootViper == nil {
		return
	}
	hints := rootHints.Lookup(cmdName)
	if len(hints) == 0 {
		return
	}
	output.RenderHints(os.Stdout, hints, formatFromViper(), rootViper, hintMuted)
}

// State toggled by RunE bodies for conditional hints. Single-shot CLI
// process so plain package globals are fine.
var (
	doctorHadFailure bool
	resumeLastUSPID  string
	listEmptyResult  bool
)

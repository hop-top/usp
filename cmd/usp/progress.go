package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"hop.top/kit/go/console/cli"
	"hop.top/kit/go/console/output"
	"hop.top/kit/go/console/progress"
	"hop.top/kit/go/console/tui"
)

const sessionSpinnerDelay = 250 * time.Millisecond

func runWithProgress(
	ctx context.Context,
	phase string,
	item string,
	fn func() error,
) error {
	stop := startSpinner(activeRoot, item)
	reporter := progress.FromContext(ctx)
	if stop == nil {
		reporter.Emit(ctx, progress.Event{Phase: phase, Item: item})
	}

	err := fn()
	ok := err == nil
	if stop != nil {
		stop()
	} else {
		reporter.Emit(ctx, progress.Event{Phase: phase, Item: item, OK: &ok})
	}
	return err
}

func startSpinner(root *cli.Root, label string) func() {
	if root == nil || root.Streams == nil || rootViper.GetBool("quiet") {
		return nil
	}
	if formatFromViper() == output.JSON || rootViper.GetString("progress-format") == "json" {
		return nil
	}
	w := root.Streams.Human
	f, ok := w.(*os.File)
	if !ok || !isatty.IsTerminal(f.Fd()) {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		spin := tui.NewSpinner(root.Theme)
		delay := time.NewTimer(sessionSpinnerDelay)
		defer delay.Stop()

		select {
		case <-ctx.Done():
			return
		case <-delay.C:
		}

		ticker := time.NewTicker(spin.Spinner.FPS)
		defer ticker.Stop()
		renderSpinnerFrame(w, spin.View(), label)
		defer clearSpinnerLine(w)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				spin, _ = spin.Update(spin.Tick())
				renderSpinnerFrame(w, spin.View(), label)
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

func renderSpinnerFrame(w io.Writer, frame string, label string) {
	_, _ = fmt.Fprintf(w, "\r%s %s", frame, label)
}

func clearSpinnerLine(w io.Writer) {
	_, _ = fmt.Fprint(w, "\r\033[2K")
}

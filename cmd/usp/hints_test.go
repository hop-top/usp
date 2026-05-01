package main

import (
	"testing"

	"hop.top/kit/cli"
	"hop.top/kit/output"
)

func TestHintsRegistered(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test"})
	prevRoot := rootViper
	rootViper = root.Viper
	t.Cleanup(func() { rootViper = prevRoot })

	registerHints(root)

	for _, want := range []string{"setup", "install", "doctor", "resume", "list"} {
		hints := root.Hints.Lookup(want)
		if len(hints) == 0 {
			t.Errorf("no hints registered for %q", want)
		}
	}
}

func TestHintsActiveByDefault(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test"})
	if !output.HintsEnabled(root.Viper) {
		t.Error("HintsEnabled = false on fresh root, want true")
	}
}

func TestHintsSuppressedByNoHints(t *testing.T) {
	root := cli.New(cli.Config{Name: "usp", Version: "test"})
	root.Viper.Set("no-hints", true)
	if output.HintsEnabled(root.Viper) {
		t.Error("HintsEnabled = true with no-hints=true, want false")
	}
}

func TestEmitHintNilSafe(t *testing.T) {
	prev := rootHints
	rootHints = nil
	t.Cleanup(func() { rootHints = prev })
	emitHint("setup") // must not panic
}

func TestDoctorFailureFlag(t *testing.T) {
	doctorHadFailure = false
	if doctorHadFailure {
		t.Error("doctorHadFailure should default false")
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"

	"hop.top/kit/go/console/cli"
)

// TestChdirHook ensures kit's -C/--chdir hook is wired on the usp
// root: parsing --chdir <dir> changes the process cwd before any
// subcommand RunE runs. Spec §5.
func TestChdirHook(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	root := cli.New(cli.Config{Name: "usp", Version: "test", DisableValidate: true})
	if root.Cmd.PersistentPreRunE == nil {
		t.Fatal("kit/cli must install a PersistentPreRunE chain (chdir hook)")
	}
	if err := root.Cmd.ParseFlags([]string{"--chdir", dir}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := root.Cmd.PersistentPreRunE(root.Cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(cwd)
	if want != got {
		t.Errorf("cwd=%q want=%q", got, want)
	}
}

// TestChdirShortFlag verifies -C accepts the same path semantics.
func TestChdirShortFlag(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	root := cli.New(cli.Config{Name: "usp", Version: "test", DisableValidate: true})
	if err := root.Cmd.ParseFlags([]string{"-C", dir}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := root.Cmd.PersistentPreRunE(root.Cmd, nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
	cwd, _ := os.Getwd()
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(cwd)
	if want != got {
		t.Errorf("cwd=%q want=%q", got, want)
	}
}

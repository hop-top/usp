package main

import (
	"errors"
	"testing"
)

func TestExitErrUnwrap(t *testing.T) {
	inner := errors.New("not found")
	ee := exitNotFound(inner)
	if ee.Error() != "not found" {
		t.Errorf("Error() = %q, want %q", ee.Error(), "not found")
	}
	var got *exitErr
	if !errors.As(ee, &got) {
		t.Fatal("errors.As failed")
	}
	if got.code != 3 {
		t.Errorf("code = %d, want 3", got.code)
	}
	if !errors.Is(ee, inner) {
		t.Error("errors.Is(ee, inner) = false")
	}
}

func TestSessionShowMissingIDExit3(t *testing.T) {
	cmd := sessionShowCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cmd.SetArgs([]string{"this-id-does-not-exist-anywhere"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	var ee *exitErr
	if !errors.As(err, &ee) {
		t.Fatalf("expected *exitErr, got %T: %v", err, err)
	}
	if ee.code != 3 {
		t.Errorf("exit code = %d, want 3", ee.code)
	}
}

func TestSessionLineageMissingIDExit3(t *testing.T) {
	cmd := sessionLineageCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// Set XDG_STATE_HOME to a temp dir so lineage.Open fails over to
	// the native fallback path which then yields not-found.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	cmd.SetArgs([]string{"this-id-does-not-exist-anywhere"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	var ee *exitErr
	if !errors.As(err, &ee) {
		t.Fatalf("expected *exitErr, got %T: %v", err, err)
	}
	if ee.code != 3 {
		t.Errorf("exit code = %d, want 3", ee.code)
	}
}

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCmd_Output(t *testing.T) {
	cmd := versionCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := buf.String()
	if !strings.HasPrefix(got, "usp v") {
		t.Errorf("output = %q, want prefix 'usp v'", got)
	}
}

func TestVersionCmd_NoArgs(t *testing.T) {
	cmd := versionCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"extra"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with positional arg")
	}
}

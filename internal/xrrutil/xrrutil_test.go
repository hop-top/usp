package xrrutil

import (
	"testing"
)

func TestActiveUnset(t *testing.T) {
	t.Setenv("XRR_MODE", "")
	if Active() {
		t.Fatal("Active() should be false when XRR_MODE is unset")
	}
}

func TestActiveSet(t *testing.T) {
	t.Setenv("XRR_MODE", "record")
	if !Active() {
		t.Fatal("Active() should be true when XRR_MODE is set")
	}
	if got := Mode(); got != "record" {
		t.Fatalf("Mode() = %q, want %q", got, "record")
	}
}

func TestCassetteDir(t *testing.T) {
	t.Setenv("XRR_CASSETTE_DIR", "/tmp/cassettes")
	if got := CassetteDir(); got != "/tmp/cassettes" {
		t.Fatalf("CassetteDir() = %q, want %q", got, "/tmp/cassettes")
	}
}

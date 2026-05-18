package id

import (
	"strings"
	"testing"
)

func TestEncodeFromNativeUUIDDeterministic(t *testing.T) {
	const native = "12341234-1234-4234-9234-123412341234"
	a, err := EncodeFromNative(PrefixSession, native)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	b, err := EncodeFromNative(PrefixSession, native)
	if err != nil {
		t.Fatalf("encode (2nd): %v", err)
	}
	if a != b {
		t.Fatalf("non-deterministic: %s vs %s", a, b)
	}
	if !strings.HasPrefix(a, PrefixSession+"_") {
		t.Fatalf("missing prefix: %q", a)
	}
}

func TestEncodeFromNativeNonUUIDDeterministic(t *testing.T) {
	a, _ := EncodeFromNative(PrefixSession, "codex-sess-01")
	b, _ := EncodeFromNative(PrefixSession, "codex-sess-01")
	if a != b {
		t.Fatalf("non-deterministic: %s vs %s", a, b)
	}
	c, _ := EncodeFromNative(PrefixSession, "codex-sess-02")
	if a == c {
		t.Fatalf("collision across distinct natives: %s", a)
	}
}

func TestEncodeFromNativePrefixIsolated(t *testing.T) {
	sessTID, _ := EncodeFromNative(PrefixSession, "shared-id")
	turnTID, _ := EncodeFromNative(PrefixTurn, "shared-id")
	// Different prefixes must produce different suffix bytes too —
	// otherwise IsTypeID + plain string compare would alias.
	if strings.TrimPrefix(sessTID, PrefixSession+"_") ==
		strings.TrimPrefix(turnTID, PrefixTurn+"_") {
		t.Fatalf("suffix collision across prefixes")
	}
}

func TestEncodeFromNativeUUIDPreservesBytes(t *testing.T) {
	const native = "12341234-1234-4234-9234-123412341234"
	tid, err := EncodeFromNative(PrefixSession, native)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	prefix, _, err := Decode(tid)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if prefix != PrefixSession {
		t.Fatalf("prefix = %q, want %q", prefix, PrefixSession)
	}
}

func TestIsTypeID(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"sess_01h455vb4pex5vsknk084sn02q", true},
		{"turn_00041061050r3gg28a1c60t3gf", true},
		{"12341234-1234-4234-9234-123412341234", false},
		{"sess_too_short", false},
		{"_01h455vb4pex5vsknk084sn02q", false},
		{"", false},
		{"SESS_01h455vb4pex5vsknk084sn02q", false}, // uppercase prefix
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := IsTypeID(tt.in); got != tt.want {
				t.Errorf("IsTypeID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"12341234-1234-4234-9234-123412341234", true},
		{"01900000-0000-7000-8000-000000000000", true},
		{"sess_01h455vb4pex5vsknk084sn02q", false},
		{"codex-sess-01", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := IsUUID(tt.in); got != tt.want {
				t.Errorf("IsUUID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestEncodeFromNativeRejectsEmpty(t *testing.T) {
	if _, err := EncodeFromNative("", "x"); err == nil {
		t.Error("expected error for empty prefix")
	}
	if _, err := EncodeFromNative(PrefixSession, ""); err == nil {
		t.Error("expected error for empty native")
	}
}

func TestDecodeRoundTrip(t *testing.T) {
	tid, err := EncodeFromNative(PrefixSession, "codex-sess-01")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	prefix, suffix, err := Decode(tid)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if prefix != PrefixSession {
		t.Errorf("prefix = %q, want %q", prefix, PrefixSession)
	}
	if len(suffix) != 26 {
		t.Errorf("suffix len = %d, want 26", len(suffix))
	}
}

func TestDecodeInvalid(t *testing.T) {
	if _, _, err := Decode("not-a-typeid"); err == nil {
		t.Error("expected error decoding garbage")
	}
}

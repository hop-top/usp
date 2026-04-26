package e2e

import (
	"strings"
	"testing"

	"hop.top/usp/internal/id"
	"hop.top/usp/internal/sessionutil"
	"hop.top/usp/session"
)

// TestTypeIDRoundTrip verifies the resolver accepts both ID forms
// and that the same session can be retrieved by either handle. This
// is the explicit guarantee called out in T-0073: any user script
// passing a native UUID must keep working, and the new TypeID form
// must also work.
func TestTypeIDRoundTrip(t *testing.T) {
	home := t.TempDir()

	claudeA := setupClaude(t, home)
	geminiA := setupGemini(t, home)
	codexA := setupCodex(t, home)
	openA := setupOpenCode(t, home)

	adapters := map[string]session.SessionAdapter{
		"claude":   claudeA,
		"gemini":   geminiA,
		"codex":    codexA,
		"opencode": openA,
	}
	order := []string{"claude", "codex", "opencode", "gemini"}

	// Pull each adapter's first session so we can resolve by both
	// its TypeID and its NativeID.
	tests := []struct {
		name   string
		native string
	}{
		{"claude", "claude-sess-01"},
		{"codex", "codex-sess-01"},
		{"opencode", "oc-sess-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_native", func(t *testing.T) {
			sess, _, _, err := sessionutil.ResolveSessionID(
				tt.native, adapters, order)
			if err != nil {
				t.Fatalf("native lookup: %v", err)
			}
			if sess.NativeID != tt.native {
				t.Errorf("NativeID = %q, want %q",
					sess.NativeID, tt.native)
			}
			if !strings.HasPrefix(sess.ID, id.PrefixSession+"_") {
				t.Errorf("ID = %q, want %s_…",
					sess.ID, id.PrefixSession)
			}
		})

		t.Run(tt.name+"_typeid", func(t *testing.T) {
			tid, err := id.EncodeFromNative(id.PrefixSession, tt.native)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			sess, _, _, err := sessionutil.ResolveSessionID(
				tid, adapters, order)
			if err != nil {
				t.Fatalf("typeid lookup: %v", err)
			}
			if sess.NativeID != tt.native {
				t.Errorf("NativeID = %q, want %q",
					sess.NativeID, tt.native)
			}
			if sess.ID != tid {
				t.Errorf("ID = %q, want %q", sess.ID, tid)
			}
		})

		// Existing prefix-match UX (T-0061) must still resolve native
		// UUIDs by their leading characters.
		t.Run(tt.name+"_native_prefix", func(t *testing.T) {
			if len(tt.native) < 8 {
				t.Skip("native too short for meaningful prefix")
			}
			prefix := tt.native[:8]
			sess, _, _, err := sessionutil.ResolveSessionID(
				prefix, adapters, order)
			if err != nil {
				t.Fatalf("prefix lookup: %v", err)
			}
			if sess.NativeID != tt.native {
				t.Errorf("NativeID = %q, want %q",
					sess.NativeID, tt.native)
			}
		})
	}
}

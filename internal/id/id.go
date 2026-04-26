// Package id wraps go.jetify.com/typeid with helpers for encoding
// usp's normalized session, turn, and tool-call identifiers.
//
// usp is read-only over native CLI session stores — TypeIDs are
// derived deterministically from the native ID at the adapter
// layer so the same native id always maps to the same TypeID
// across processes.
//
// For UUID-shaped natives (Claude UUIDv4, Codex UUIDv7) the 16
// bytes are re-used directly, preserving k-sortability. For
// non-UUID natives (e.g. OpenCode "ses_…", Codex string IDs) a
// SHA-256(prefix||native) is folded to 16 bytes and stamped with
// UUIDv7-compatible version+variant bits before encoding.
package id

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.jetify.com/typeid"
)

// Canonical prefixes for usp identifiers.
const (
	PrefixSession = "sess"
	PrefixTurn    = "turn"
	PrefixTool    = "tool"
)

// reTypeID matches the textual TypeID shape: <prefix>_<26 base32 chars>.
// Prefix is lowercase ASCII letters (typeid spec restricts to a-z and _).
var reTypeID = regexp.MustCompile(`^[a-z]([a-z_]{0,61}[a-z])?_[0123456789abcdefghjkmnpqrstvwxyz]{26}$`)

// reHexUUID matches RFC 4122 hex UUIDs (any version) with dashes.
var reHexUUID = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

// IsTypeID reports whether s has the textual shape of a TypeID.
// Cheap regex check — does not validate the suffix is a real UUID.
func IsTypeID(s string) bool { return reTypeID.MatchString(s) }

// IsUUID reports whether s is an RFC 4122 hex UUID with dashes.
func IsUUID(s string) bool { return reHexUUID.MatchString(s) }

// EncodeFromNative returns a deterministic TypeID for a native ID.
// Same native ID + prefix always produces the same TypeID.
//
// When native is a hex UUID, its 16 bytes are reused so UUIDv7
// inputs stay k-sortable. Otherwise SHA-256(prefix||native) is
// folded to 16 bytes and stamped with UUIDv7 version+variant
// bits before encoding.
func EncodeFromNative(prefix, native string) (string, error) {
	if prefix == "" {
		return "", errors.New("id: empty prefix")
	}
	if native == "" {
		return "", errors.New("id: empty native id")
	}
	bytes, err := canonicalBytes(prefix, native)
	if err != nil {
		return "", err
	}
	tid, err := typeid.FromUUIDBytesWithPrefix(prefix, bytes)
	if err != nil {
		return "", fmt.Errorf("id: encode %s_%x: %w", prefix, bytes, err)
	}
	return tid.String(), nil
}

// canonicalBytes returns the 16-byte UUID payload that backs the
// TypeID for the given native id. Hex UUIDs round-trip through
// their bytes; everything else goes through SHA-256 with the
// prefix mixed in to avoid collisions across id classes.
func canonicalBytes(prefix, native string) ([]byte, error) {
	if IsUUID(native) {
		raw, err := hex.DecodeString(strings.ReplaceAll(native, "-", ""))
		if err != nil {
			return nil, fmt.Errorf("id: decode uuid %q: %w", native, err)
		}
		if len(raw) != 16 {
			return nil, fmt.Errorf("id: uuid %q decoded to %d bytes", native, len(raw))
		}
		return raw, nil
	}
	sum := sha256.Sum256([]byte(prefix + "\x00" + native))
	out := make([]byte, 16)
	copy(out, sum[:16])
	out[6] = (out[6] & 0x0f) | 0x70 // UUIDv7 version
	out[8] = (out[8] & 0x3f) | 0x80 // RFC 4122 variant
	return out, nil
}

// Decode parses s as a TypeID and returns its prefix and base32 suffix.
func Decode(s string) (prefix, suffix string, err error) {
	tid, err := typeid.FromString(s)
	if err != nil {
		return "", "", fmt.Errorf("id: decode %q: %w", s, err)
	}
	return tid.Prefix(), tid.Suffix(), nil
}

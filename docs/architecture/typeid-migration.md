# TypeID Migration

Last run: 2026-04-26
Task: T-0073
Author: $USER

Decision record for replacing raw native session UUIDs with prefixed
TypeIDs (`sess_…`, `turn_…`, `tool_…`) at the usp envelope layer.

## Context

usp aggregates session metadata across four CLI vendors (Claude,
Codex, Gemini, OpenCode), each with its own ID scheme:

| CLI       | Native ID format                        |
|-----------|-----------------------------------------|
| Claude    | UUIDv4 hex with dashes                  |
| Codex     | UUIDv7 hex with dashes                  |
| Gemini    | free-form `<tag>` from `/chat save`     |
| OpenCode  | `ses_<26 base32 chars>` (TypeID-shaped) |

T-0061 patched the partial-prefix UX symptom (`session show <8-char prefix>`
returned ambiguous-prefix errors against full UUIDs). The structural
issue remained: identifiers were opaque UUIDs that could collide with
each other across CLIs and gave users no class hint (session vs turn
vs tool).

## Decision

Adopt [TypeID](https://github.com/jetify-com/typeid) v1.3.0
(`go.jetify.com/typeid`) as the canonical envelope-level identifier
for sessions, turns, and tool calls.

- Internal `session.Session` exposes two id fields:
  - `ID` — canonical TypeID (e.g. `sess_01h455vb4pex5vsknk084sn02q`).
  - `NativeID` — the underlying CLI session id (UUID, `ses_…`, tag).
- A new `internal/id` package wraps `typeid.FromUUIDBytesWithPrefix`
  with `EncodeFromNative(prefix, native)`:
  - UUID natives reuse their 16 bytes verbatim — UUIDv7 inputs stay
    k-sortable.
  - Non-UUID natives (Codex pre-uuid IDs, Gemini tags, OpenCode
    `ses_…`) fold through `SHA-256(prefix || \x00 || native)` with
    UUIDv7 version+variant bits stamped before encoding.
- Adapters call `Session.SetIDs(native)` to populate both fields.
- `sessionutil.ResolveSessionID` accepts either form: TypeID and
  native are matched against `Session.ID` and `Session.NativeID`
  respectively, and `IsTypeID` short-circuits the GetSession fast
  path so adapter file/db lookups always receive native ids.

## Why TypeID over alternatives

- **Plain UUIDv7 with prefix string**: gives type info but loses
  the canonical text form, and we'd hand-roll the base32 codec.
- **Custom scheme (`usp:<cli>:<id>`)**: works internally but doesn't
  align with the broader `hop.top/uri` direction and lacks an
  external standard.
- **uxid / xid / nanoid**: lack the prefix discriminator that makes
  TypeIDs self-describing in CLI output.

TypeID is small (single dep), Apache 2.0, and the spec is stable
(v0.3 since 2023). The Go binding has been at v1.3.0 since Jul 2024.

## Backward compatibility

- `session show <native-uuid>` still works — the resolver tries
  native fast-path first when input is not TypeID-shaped.
- T-0061 prefix-match UX still works — the prefix scan now compares
  against both `s.ID` and `s.NativeID`.
- `session list --format json` includes both `id` (TypeID) and
  `native_id`, so scripts that grep for native UUIDs keep working.
- Vendor session files on disk are never mutated. usp stays read-only.

## Rollout

1. Land `internal/id` + envelope changes (this PR).
2. Update README session snippets to show TypeID examples; keep
   the native-id examples as a "still works" footnote.
3. Update `~/.agents/AGENTS.md` `usp` section once shipped — drop the
   "Requires full UUID" caveat.
4. Future: consider TypeIDs for turns and tool calls in the
   normalized envelope (currently only sessions are surfaced
   through user-facing commands).

## Out of scope

- Mutating CLI vendor session files (usp stays read-only).
- TypeIDs for ctxt or tlc identifiers — those are separate
  per-project decisions.
- Server-side stable session storage; lineage already keys by
  native id and that's fine.

## Verification

- Unit tests in `internal/id` cover deterministic encoding,
  cross-prefix isolation, UUID byte preservation, and shape
  detection.
- Resolver tests in `internal/sessionutil` cover both ID forms
  and the T-0061 prefix-match path.
- Integration test `e2e/typeid_roundtrip_test.go` walks each
  adapter's fixture and looks the same session up by TypeID,
  by native id, and by native prefix.

## References

- TypeID spec: https://github.com/jetify-com/typeid
- Go impl: https://pkg.go.dev/go.jetify.com/typeid
- T-0061 (partial-prefix UUID match — Done) — UX precursor
- T-0073 (this migration)

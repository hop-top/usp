# Session tools-usage view (design)

Author: $USER
Status: draft (brainstorm output, T-0072)
Supersedes: original ctxt-only scope in
`docs/personas/individuals/platform-engineer.md` (4th user story)
Related: T-0070 (skills), T-0071 (agents)

## TL;DR

Add a single `usp session tools` subcommand that surfaces every
tool invocation in matching sessions, classified by `category`
(native / bash / mcp / skill / agent), `backing` (binary or MCP
server), and `name`. Outcome classification splits along two
heuristic families: **lookup-tools** (relevance heuristic:
`conclusive | partial | noise | unknown`) and **action-tools**
(execution heuristic: `success | fail | unknown`). T-0070 and
T-0071 stay as dedicated subcommands but become thin filtered
views over the same classifier; the same data is reachable via
`tools --category skill` or `--category agent`.

## Context

Original scope ("list ctxt queries + outcomes") was too narrow.
ctxt is one of many tools invoked from inside a session: Bash
subcommands, MCP servers, native Claude tools (Read/Edit/Grep),
Task/Skill/WebFetch/WebSearch, agent dispatches. Shipping a
ctxt-only surface buys a one-off feature that we'd then re-
architect when we want gh/tlc/MCP coverage. This doc resolves
the open questions in T-0072 and produces a concrete follow-up
task list.

## Tasks

| Task | Title | Effort | Priority | Blocked-by |
|---|---|---|---|---|
| TF-1 | feat(session): tool classifier + ToolEvent envelope | M | P1 | T-0070, T-0071 |
| TF-2 | feat(session): `usp session tools` subcommand | M | P1 | TF-1 |
| TF-3 | feat(session): `--tools` flag on `session show` | S | P1 | TF-2 |
| TF-4 | refactor(session): `skills` + `agents` views over classifier | S | P2 | TF-1, T-0070, T-0071 |
| TF-5 | docs(session): tool taxonomy reference + adapter contract | S | P2 | TF-1 |

Detailed specs in [§7. Concrete follow-up task list](#7-concrete-follow-up-task-list).

---

## 1. Definition of "tool"

**Decision.** A "tool" is any structured invocation that appears
as a `ToolCall` in the normalized session envelope
(`session.ToolCall{Name, Input, Output, ID}`). Classification is
derived from `Name`, with sub-extraction of leading binary for
`Bash` and server/tool split for MCP.

**Categories (top-level partition):**

| Category | Match rule on `ToolCall.Name` | Examples |
|---|---|---|
| `native` | Built-in Claude tool name | `Read`, `Edit`, `Write`, `Grep`, `Glob`, `NotebookEdit`, `WebFetch`, `WebSearch` |
| `bash` | `Name == "Bash"`; sub-extract leading binary from `Input.command` | `ctxt …`, `gh …`, `tlc …`, `make …`, `git …` |
| `mcp` | `Name` matches `^mcp__<server>__<tool>$` | `mcp__plugin_playwright_playwright__browser_click` |
| `skill` | `Name == "Skill"` (or first content block `<command-name>` match in claude transcripts) | `superpowers:writing-plans`, `feature-dev:code-reviewer` |
| `agent` | `Name == "Task"` | subagent dispatches with `subagent_type` in input |

**Why not "by leading binary" for everything (rejected
alternative).** Treating every Bash subcommand on equal footing
with native tools collapses three different observability
problems: (a) "what filesystem actions did Claude perform"
(native), (b) "what shell programs did it invoke" (bash),
(c) "what MCP servers does this engineer rely on" (mcp). The
category dimension preserves all three; users who want a flat
view ask for `--category bash` or omit `--category` entirely
and pivot on `backing`.

**Why not "per-tool only" (rejected alternative).** Per-tool
flattening (one row per `Bash binary`, one row per MCP tool
name) is the *output*, not the *taxonomy*. The classifier
emits per-row events; grouping happens at render time via
`--exec group_by:<col>`. Conflating taxonomy and aggregation
makes the classifier non-reusable for `session show`'s
single-session detail mode.

**Boundary cases (resolved):**

- **Skill via `<command-name>` block in claude transcripts.** Claude
  emits skills both via the `Skill` tool call and via inline
  `<command-name>` blocks in turn content. The classifier MUST
  recognise both shapes and emit a `category=skill` event with
  consistent fields. Adapter-level normalisation (claude-only)
  converts `<command-name>` content blocks into synthetic
  `ToolCall{Name: "Skill", Input: {name: ...}}` records before the
  classifier sees them, so the classifier itself stays
  CLI-agnostic.
- **Agent dispatches.** `Task` tool calls are a strict subset of
  the unified surface. T-0071 `usp session agents` becomes a
  filtered view (`--category agent`) but keeps its dedicated
  subcommand for ergonomics (see §4).
- **WebFetch / WebSearch.** Classified as `native` with
  `family=lookup`. Outcome heuristic uses returned-content
  presence + assistant follow-up reuse (same heuristic as ctxt,
  see §3).
- **Read / Grep / Glob.** Classified as `native` with
  `family=lookup`. Outcome heuristic same as ctxt.
- **Edit / Write / NotebookEdit.** Classified as `native` with
  `family=action`. Outcome derived from tool_result success/
  failure, not relevance.

---

## 2. Surface shape

**Decision.** Single `usp session tools` subcommand with
`--category`, `--backing`, `--name`, `--family`, `--outcome`
filters. Skills and agents views (T-0070, T-0071) ship as
dedicated subcommands but become thin aliases over the same
classifier (see §4).

**Why single subcommand (chosen).**

- Filter surface is identical across categories
  (`--session/--cli/--project/--since/--until` + classification
  filters). A single command has one help page, one schema, one
  set of acceptance criteria.
- `--exec` (kit-level executor pipeline) is uniform across
  categories — `summarize:tokens(N)`, `count`, `group_by:<col>`,
  embedded `filter(...)` all work without per-category special-
  casing.
- New tool categories (e.g. future `mcp-resource`) extend the
  classifier without adding subcommands.
- Output schema unified: every row carries the same envelope
  (`session_id, cli, ts, category, backing, name, family,
  outcome, trigger_query, …`); JSON consumers parse once.

**Why not per-category subcommands (rejected for `tools`,
retained for `skills`/`agents` aliases).** `usp session bash`,
`usp session mcp`, `usp session native` would proliferate
commands with near-identical flags and force callers to know the
category before querying. Skills and agents stay as dedicated
subcommands only because they ship as part of the
session-introspection track and their persona stories
(US-0005) reference the named commands directly; demoting them
to filter values would invalidate already-published ACs and
break the persona-driven UX.

**Why not `session show <id> --classification …` only (rejected
alternative).** `--classification` only fits the lookup family
(see §3). A single flag forced across both families either
over-promises (`--classification success` is meaningless for
lookups) or under-delivers (lookup-only flag ignores half the
data). Two-flag split (`--family` + `--outcome`) lets the user
ask category-coherent questions.

**Subcommand surface (locked):**

```
usp session tools [filters] [--format table|json|yaml]
usp session tools --category bash --backing ctxt
usp session tools --category mcp --backing playwright
usp session tools --family lookup --outcome conclusive
usp session tools --since 7d --exec count
usp session tools --since 7d --exec 'group_by:backing'

usp session show <id> --tools                 # embed all tool events inline
usp session show <id> --tools --category bash # embed filtered subset

# Skills/agents views (delegated to T-0070/T-0071, filtered alias):
usp session skills [filters]    # = tools --category skill, with skill-specific cols
usp session agents [filters]    # = tools --category agent, with agent-specific cols
```

Diagram: see [`session-tools-view/surface-v1.mmd`](session-tools-view/surface-v1.mmd).

---

## 3. Classification taxonomy

**Decision.** Two heuristic families, both attached to every
`ToolEvent` row:

```
family   ∈ { lookup, action, mixed }
outcome:
  family=lookup → outcome ∈ { conclusive, partial, noise, unknown }
  family=action → outcome ∈ { success,    fail,    unknown }
  family=mixed  → outcome ∈ { success,    fail,    unknown }   // strict on tool_result
```

**Why two families (chosen).** The
`conclusive/partial/noise/unknown` heuristic is meaningful only
for tools whose primary purpose is to retrieve information for
the assistant to *decide* with. For tools that mutate state
(`Edit`, `Write`, `git push`, `make build`, MCP `save_issue`),
the relevant signal is *did it execute cleanly*, derived
mechanically from `tool_result`'s success indicator (exit code,
error field, status). Forcing one heuristic across both produces
nonsense (`Edit` returning `noise`?) or hides genuine failures
(`gh pr create` succeeded but assistant ignored the URL — not a
"noise" lookup, it's an unused write).

**Why not a single heuristic (rejected alternative).** A unified
"useful / not useful" heuristic either reduces to the lookup
heuristic (and loses action-tool fidelity) or to the action
heuristic (and loses the relevance signal that's the *whole
point* of the original ctxt-only scope). Splitting is the only
shape that preserves both signals.

**Mapping by category:**

| Category | family default | Notes |
|---|---|---|
| `native` Read/Grep/Glob/WebFetch/WebSearch/NotebookEdit-read | `lookup` | Outcome from assistant follow-up (see Lookup heuristic) |
| `native` Edit/Write/NotebookEdit-write | `action` | Outcome from tool_result success/error |
| `bash` (per-binary) | per-binary table (below) | `mixed` for binaries with both subcommands (e.g. `git`) |
| `mcp` (per-tool) | per-tool default; falls back to `mixed` | Linear `get_*` = lookup; `save_*` = action |
| `skill` | `action` | Outcome = invoked / declined / errored (strict); separate skill-specific schema |
| `agent` | `action` | Outcome = completed / errored / cancelled (strict); separate agent-specific schema |

**Per-binary defaults for `bash`** (initial seed, extensible via
config):

| Binary | family | Notes |
|---|---|---|
| `ctxt`, `rsx`, `xray`, `usp`, `gh issue list`, `gh pr view`, `tlc task show`, `tlc task list` | `lookup` | Pure read |
| `make`, `pnpm`, `npm`, `cargo`, `go test`, `go build`, `git push`, `git commit`, `gh pr create`, `tlc task create`, `tlc task complete`, `rm`, `trash` | `action` | Side-effects |
| `git` (top-level), `tlc` (top-level), `gh` (top-level) | `mixed` | Sub-extract second token; if not in table, default `action` (safer) |

The mapping table lives in `session/classify.go` as a default
that can be augmented by an optional `~/.config/usp/tools.yaml`
override. Out of scope for the brainstorm; flagged as a
follow-up tweak in TF-5.

**Lookup heuristic (relevance) — definitions:**

Window for all definitions: the next 3 assistant turns after the
tool_result. "Similar input" = same tool name + ≥ 50% token
overlap on input string.

- `conclusive` — within window, assistant references the tool's
  output (substring or near-paraphrase, ≥ 1 token match) AND
  does not retry the same tool with similar input.
- `partial` — within window, assistant references output AND
  retries the same tool with similar input.
- `noise` — within window, assistant references nothing from
  the output AND does not retry. The output was discarded.
- `unknown` — fewer than 1 assistant turn exists after the
  tool_result (session truncated / ended).

**Errored lookup calls.** When a lookup-family tool returns an
error indicator (claude `is_error: true`, non-zero exit, mcp
`error`), the classifier promotes the row to
`family=mixed, outcome=fail`. Lookup outcomes stay clean
(relevance only); errors live in the action enum. This keeps
the lookup outcome set minimal and lets `--family lookup
--outcome noise` mean exactly "tool returned data, assistant
ignored it" with no error contamination.

**Action heuristic — definitions:**

- `success` — `tool_result` carries no error indicator (claude:
  no `is_error: true`; bash: exit code 0; mcp: no `error` field).
- `fail` — explicit error indicator (claude `is_error: true`;
  non-zero exit; mcp `error` populated).
- `unknown` — `tool_result` missing (session truncated mid-call)
  OR error indicator unparseable.

**Why the lookup heuristic generalizes beyond ctxt.** It only
depends on (a) post-call assistant content, (b) presence/
absence of follow-up calls with similar input. Both are
adapter-agnostic signals already in the envelope. Concretely
proven for `ctxt find`, `gh issue list`, `Grep`, `WebFetch` —
all match the same shape: tool returns objects, assistant either
uses them or not.

**Schema (ToolEvent envelope, per row):**

```pseudocode
ToolEvent {
  session_id     string
  cli            string
  ts             time
  category       enum (native | bash | mcp | skill | agent)
  backing        string                       # binary / mcp server / "claude" / skill plugin
  name           string                       # tool name (or sub-binary for bash)
  family         enum (lookup | action | mixed)
  outcome        enum (conclusive | partial | noise | unknown |
                       success    | fail    | unknown)
  trigger_turn_id string
  trigger_query   string                      # truncated input (or surrounding turn content)
  result_summary  string                      # truncated output / error
  duration_ms     int                         # if available
  unsupported     bool                        # CLI lacks the primitive
}
```

**Skills + agents subset rows.** When rendering via the
dedicated `skills` / `agents` subcommands (T-0070, T-0071), the
renderer projects the `ToolEvent` into a category-specific row
shape that adds `skill_name` / `agent_type` / `agent_name` /
`agent_description` etc. (already specified in T-0070/T-0071
descriptions). The underlying classifier output is the same
ToolEvent stream.

Diagram: see [`session-tools-view/taxonomy-v1.mmd`](session-tools-view/taxonomy-v1.mmd).

---

## 4. Relationship to T-0070 (skills) and T-0071 (agents)

**Decision.** Skills and agents stay as dedicated subcommands
(`usp session skills`, `usp session agents`) but reuse the
unified classifier internals. They become **thin filtered
views** over `session tools`, NOT separate code paths.

**Why retain dedicated subcommands.**

- US-0005 ACs reference `usp session skills` and `usp session
  agents` by name; persona stories (Solo Developer, AI Agent,
  Platform Engineer) ask the questions at category granularity.
  Demoting to `tools --category skill` makes the persona-facing
  command longer than necessary for a narrative-validated query.
- Output columns differ per category (skills carry
  `skill_name + outcome`; agents carry `agent_type + agent_name +
  duration_ms + exit_status`). A unified `tools` row exposes
  them as `name + family + outcome` etc., which is correct but
  loses ergonomic per-category column naming. The dedicated
  subcommands project to the persona-tuned shape.
- Build sequence: T-0070 ships first (per
  `session-introspection/plan.md`), and the unified classifier
  (TF-1) ships AFTER. The skills view's adapter-level skill
  detection scaffolding feeds straight into the classifier's
  `category=skill` branch — TF-4 then refactors T-0070's render
  path to consume the classifier instead of its own bespoke
  parser, with no externally visible change.

**Why not collapse to filtered views only (rejected
alternative).** Forcing users to type
`usp session tools --category skill --format json | jq …` for
the most common audit query (the one shipped first, with the
biggest narrative weight in personas) is a usability
regression versus the dedicated `skills` command. Single-source-
of-truth is preserved by the shared classifier; the wrapper
subcommands are 30 lines of cobra glue each.

**Implementation order:**

1. T-0070 (skills) — ships first as planned, with its own
   detection logic; output passes through `session.ToolCall`
   into a skill-specific renderer.
2. T-0071 (agents) — same pattern.
3. TF-1 — extracts a `Classifier` from the union of
   T-0070 + T-0071 detection logic and adds the unified
   `ToolEvent` envelope.
4. TF-2 — ships `usp session tools`, which uses the classifier
   directly.
5. TF-4 — refactors T-0070 + T-0071 internals to call the same
   classifier and project to category-specific renderers.
   Externally observable surface unchanged (passes existing e2e
   cassettes); diff is internal-only.

This avoids gating T-0070/T-0071 on TF-1 work; the classifier is
distilled FROM the skills/agents implementations rather than
preceding them.

---

## 5. Cross-CLI semantics

Same posture as T-0070/T-0071: claude is the primary, others
emit best-effort or `unsupported: true`.

| CLI | native | bash | mcp | skill | agent |
|---|---|---|---|---|---|
| claude | ToolCall name match | ToolCall name == "Bash" | ToolCall name `mcp__*` | ToolCall name == "Skill" (after adapter normalises `<command-name>`) | ToolCall name == "Task" |
| codex | partial (its tool-call records) | TBD per codex tool format | not supported | not supported | partial |
| gemini | partial | partial | not supported | not supported | partial |
| opencode | partial | partial | not supported | not supported | not supported |
| copilot | TBD (adapter not landed) | TBD | TBD | TBD | TBD |

CLIs without a primitive emit `unsupported: true` per row (same
as US-0005 AC). No hard error.

---

## 6. Open questions resolved by this doc

| Open question (from T-0072 description) | Decision | Section |
|---|---|---|
| What counts as a "tool"? | `session.ToolCall` is the primitive; classifier maps to category | §1 |
| Group by category, backing, or per-tool? | Category is the primary partition; backing + name as drill-downs; per-tool is render-time via `--exec group_by` | §1, §2 |
| Does conclusive/partial/noise/unknown generalize? | Generalizes for the **lookup family** only; action family uses success/fail | §3 |
| Single subcommand or per-category? | Single `tools` subcommand; skills/agents dedicated as filtered aliases for ergonomics | §2, §4 |
| Are skills + agents subsets of tools? | Yes, classifier-wise; UX-wise they keep dedicated subcommands | §4 |
| Could T-0070/T-0071 be filtered views? | Yes internally (TF-4); externally they retain dedicated subcommand UX | §4 |

---

## 7. Concrete follow-up task list

Track placement: **new track** `session-tools-view`. Rationale:
`session-introspection` is in flight (T-0070 IN_PROGRESS,
T-0071 TODO); adding 5 more tasks expands scope mid-execution
and would re-trigger plan review. New track keeps build
sequence clean: T-0070 → T-0071 → TF-1 → TF-2 → TF-3 → TF-4 →
TF-5.

### TF-1: feat(session): tool classifier + ToolEvent envelope

- **Effort:** M
- **Priority:** P1
- **Tags:** `type:feat, domain:session, surface:lib`
- **Blocked-by:** T-0070, T-0071 (classifier distilled FROM
  their detection logic)
- **Description outline:**
  - Add `session/classify.go` exposing
    `Classifier.Classify(ToolCall) ToolEvent` (or
    `ClassifyTurn(Turn) []ToolEvent` for `<command-name>` skill
    blocks).
  - Add `ToolEvent` envelope per §3 schema.
  - Seed per-binary mapping table for bash sub-extraction
    (default in code; loadable override in
    `~/.config/usp/tools.yaml` deferred to TF-5).
  - Lookup outcome heuristic: implement assistant-follow-up
    inspection (next 3 turns).
  - Action outcome heuristic: parse `is_error`, exit codes, mcp
    `error` field.
  - Unit tests: classification of every category × outcome
    combination using fixture transcripts.
  - No new public CLI surface; library only.

### TF-2: feat(session): `usp session tools` subcommand

- **Effort:** M
- **Priority:** P1
- **Tags:** `type:feat, domain:session, surface:cli`
- **Blocked-by:** TF-1
- **Description outline:**
  - Add `cmd/usp/session_tools.go` registering `tools`
    subcommand on `session`.
  - Filter flags: `--session, --cli, --project, --since,
    --until, --category, --backing, --name, --family, --outcome`
    + `--format`.
  - Wire kit-level `--exec` (same as T-0070/T-0071).
  - Default render: `session_id, cli, ts, category, backing,
    name, family, outcome` (table); full ToolEvent (json/yaml).
  - Per-category column sets pivot when `--category` is single-
    valued (e.g. `--category skill` adds `skill_name +
    trigger_query`).
  - E2E test `e2e/session_tools_test.go` against canned claude
    transcript fixture exercising native + bash + mcp + skill +
    agent in one session.
  - Story file `US-0006-inspect-session-tools.md` with ACs
    mirroring §2 surface.

### TF-3: feat(session): `--tools` flag on `session show`

- **Effort:** S
- **Priority:** P1
- **Tags:** `type:feat, domain:session, surface:cli`
- **Blocked-by:** TF-2
- **Description outline:**
  - Extend `cmd/usp/session_show.go` with `--tools` flag
    (mirroring `--skills`/`--agents` from T-0070/T-0071).
  - When set, embed ToolEvent rows inline with the transcript
    (sorted by `ts`); applies `--category/--backing/--name`
    filters if combined.
  - JSON output: ToolEvent rows attached to the parent showResult
    under a `tools` array.
  - Unit + e2e coverage piggybacking on TF-2's fixture.

### TF-4: refactor(session): skills + agents views over classifier

- **Effort:** S
- **Priority:** P2
- **Tags:** `type:refactor, domain:session, surface:cli`
- **Blocked-by:** TF-1, T-0070, T-0071
- **Description outline:**
  - Refactor T-0070's `session_skills.go` + T-0071's
    `session_agents.go` to call the unified Classifier instead
    of bespoke detection paths.
  - Project ToolEvent into category-specific renderer
    (skill_name, trigger_query, outcome for skills; agent_type,
    duration_ms, exit_status for agents).
  - Externally observable surface unchanged; existing e2e
    cassettes from T-0070/T-0071 must continue to pass byte-for-
    byte (replay-mode regression check).
  - No new docs (TF-5 owns docs); commit message references
    issue # for the unified design.

### TF-5: docs(session): tool taxonomy reference + adapter contract

- **Effort:** S
- **Priority:** P2
- **Tags:** `type:docs, domain:session`
- **Blocked-by:** TF-1
- **Description outline:**
  - Update this doc (`docs/architecture/session-tools-view.md`)
    from prescriptive to descriptive once TF-1/TF-2/TF-3/TF-4
    land. Replace forward-looking decisions with shipped-
    behavior references; preserve "Open questions resolved"
    table for traceability.
  - Document per-binary mapping table format and
    `~/.config/usp/tools.yaml` override schema (decision point:
    ship the override hook in TF-1 or defer; default = defer to
    TF-5 if usage emerges).
  - Update `docs/architecture.md` package layout (`session/
    classify.go`, `cmd/usp/session_tools.go`).
  - Story file: add `US-0006-inspect-session-tools.md` to
    `docs/stories/`.

### Filing notes

Filing creates all 5 tasks at once (after this brainstorm is
approved) with cross-blockers per the §7 table; build order is
enforced by `blocked-by`, not by staggered filing. Track
creation precedes ingestion:

```
tlc track create "Session tools-usage view" --type feature \
  --id session-tools-view
tlc track update session-tools-view \
  --add-plan tracks/session-tools-view/plan.md
```

T-0072 is marked complete and referenced from the new track's
plan.md as the originating brainstorm.

---

## 8. Risks + mitigations

| Risk | Mitigation |
|---|---|
| Classifier diverges from skills/agents detection if TF-4 deferred | Make TF-4 P2 not P3; track it as DoD for the surface |
| Lookup heuristic noisy on bash with cached output (e.g. `gh issue list` returning identical content twice) | Heuristic relies on assistant *content reuse*, not output equality. Reused = `conclusive`; ignored = `noise`. Verified via fixture. |
| Per-binary mapping requires per-engineer tuning | Ship default table covering top-20 binaries (ctxt/gh/tlc/git/make/cargo/go/pnpm/npm/rsx/xray/rm/trash/sed/awk/jq/curl/wget/test/echo); override file deferred to TF-5 |
| MCP server sprawl bloats `--backing` cardinality | Render-time grouping via `--exec group_by:backing`; backing column is high-cardinality but searchable |
| TF-1 schedule gated on T-0070 + T-0071 (chain T-0070 → T-0071 → TF-1 → TF-2 → TF-3) | Persona deliverables (skills + agents) ship before tools-view, so unified surface lateness is internal-refactor-only and doesn't block US-0005 ACs |

---

## 9. Verification

This doc was reviewed via `feature-dev:code-reviewer` agent;
review notes saved at
`reviews/session-tools-view-design-review.md`. Blocking issues
addressed before commit.

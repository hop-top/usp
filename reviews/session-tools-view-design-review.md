# Review: session-tools-view design doc

**Date:** 2026-04-26
**Reviewer:** self-review (T-0072 author)
**Doc:** `docs/architecture/session-tools-view.md`
**Track / task:** brainstorm T-0072 (unlinked from
session-introspection)

## Critical (must fix before commit)

### 1. TF-1 `Blocked-by` contradicts Implementation order (confidence: 0.95)

`§7` summary table (line 37) lists TF-1 `Blocked-by: (none)`,
but `§4 Implementation order` step 3 states TF-1 is "extracts a
`Classifier` from the union of T-0070 + T-0071 detection logic"
— which makes T-0070 and T-0071 hard prerequisites. The TF-1
detail block at line 400 correctly lists
`Blocked-by: T-0070, T-0071`. Top table is wrong.

**Fix:** Update line 37 cell to `T-0070, T-0071`.

### 2. TF-5 description self-references its own existing path (confidence: 0.90)

TF-5 task description says:

> Promote this brainstorm doc into a formal architecture
> reference at `docs/architecture/session-tools-view.md` (this
> file).

The doc is *already* at that path. The task should describe what
TF-5 actually *does* once implementation lands: refresh the doc
from prescriptive (decisions to make) to descriptive (decisions
made + extension surface), and add the per-binary mapping
override format.

**Fix:** Reword TF-5 first bullet to "Update this doc from
prescriptive to descriptive once TF-1/TF-2/TF-3/TF-4 land
(replace forward-looking decisions with shipped-behavior
references)."

### 3. Markdown table row exceeds 256 char hard limit (confidence: 1.0)

Line 514 (Risks table, second row) is 288 chars. Project
convention from `~/.agents/AGENTS.md`:
> EXCEPT markdown table rows: hard limit 256 chars

**Fix:** Split the cell content across two sentences; or split
into two rows.

## Important (should fix before commit)

### 4. Lookup heuristic `noise` lacks the same time bound as `partial` (confidence: 0.80)

`§3` defines:

- `partial` — assistant references output BUT issues another
  call to the same tool **within the next 3 turns**
- `noise` — assistant references nothing AND **does not call
  the same tool again** (no window specified)

A retry 50 turns later would push a result from `noise` to
`partial` under the literal reading. Inconsistent heuristic
window destabilises classification.

**Fix:** Constrain `noise` with same 3-turn window: "within the
next 3 turns, references nothing from the output AND does not
call the same tool again with similar input."

### 5. Lookup `unknown` conflates "no follow-up" with "tool errored" (confidence: 0.75)

`unknown` definition includes "session ended within 1 turn of
the call (no follow-up to inspect), or output was empty/error."
For a lookup tool, an explicit error is an action-side signal
(`fail`). Folding it into `unknown` loses the distinction
between "we never saw a follow-up" and "we know the lookup
errored." Also: "output was empty" is debatable — empty result
is information (relevant query, no hits) and conflates with
truncation.

**Fix:** Refine `unknown` to "session ended within 1 turn of the
call (no follow-up to inspect)." Add a `fail` outcome to the
lookup family for explicit error responses, OR explicitly note
that errored lookup calls render as `family=mixed, outcome=fail`
to keep the lookup outcome set clean. Recommend the latter
(family promotion on error) to keep enum minimal.

### 6. §4 build sequence vs §7 filing order is confusing (confidence: 0.70)

`§7 Filing order` says "user files TF-1 and TF-2 first (to
unblock TF-3)" — which conflates *filing* (creating tasks in
tlc, happens once after this brainstorm) with *build* (execution
order, T-0070 → TF-1 → …). They're not the same thing.

**Fix:** Rename `§7 Filing order` heading to "Filing notes" and
clarify: filing creates all 5 tasks at once with cross-blockers;
build order is enforced by `blocked-by`. No partial filing.

## Notes (non-blocking)

### 7. §5 cross-CLI table understates `<command-name>` normalisation

The claude column for skill says `ToolCall + <command-name>
block`, but §1 establishes that the claude adapter normalises
`<command-name>` blocks into synthetic `ToolCall{Name: "Skill"}`
records *before* the classifier runs. So at classifier time it's
just `ToolCall name == "Skill"`. The table conflates two layers.

**Optional fix:** Replace cell with `ToolCall name == "Skill"
(after adapter normalisation)`.

### 8. Schedule risk: TF-1 gated on T-0071 (currently TODO)

TF-1 needs T-0070 + T-0071 to ship before it can start. T-0071
is `blocked-by T-0070`, so the dependency chain is T-0070 →
T-0071 → TF-1 → TF-2 → TF-3. Worth calling out in §8 Risks if
the engineer expects the unified surface "soon."

**Optional fix:** Add a "Schedule" row to §8 risks listing this
chain and noting that nothing in the chain blocks the persona
deliverables (skills + agents both ship before tools).

### 9. Doc author placeholder `$USER`

Frontmatter line 3: `Author: $USER`. Project convention from
`~/.agents/AGENTS.md` says "MUST use $USER for author" — keep
the literal placeholder. Confirmed correct.

## Outcome

Three blocking issues (1, 2, 3) MUST be fixed before commit.
Three important issues (4, 5, 6) SHOULD be fixed for design
clarity. Three notes (7, 8, 9) are optional.

The core decisions in the doc — single `tools` subcommand, two
heuristic families, dedicated skills/agents subcommands as
filtered views with shared classifier — are sound and resolve
all open questions in T-0072. No re-design needed; only edits.

## Resolution

All blocking + important issues addressed in the same commit:

- Issue 1: TF-1 `Blocked-by` corrected to `T-0070, T-0071` in
  §7 summary table.
- Issue 2: TF-5 reworded; no longer "promotes" the doc to its
  own existing path. Now: update prescriptive → descriptive.
- Issue 3: Risk row split / shortened; no markdown table row
  exceeds 256 chars.
- Issue 4: Lookup heuristic window unified (3 assistant turns
  after tool_result); "similar input" defined.
- Issue 5: Errored lookup calls promoted to `family=mixed,
  outcome=fail` rather than collapsing into lookup `unknown`.
- Issue 6: §7 "Filing order" → "Filing notes"; clarified that
  filing happens once with cross-blockers, not staggered.

Optional notes 7 + 8 also addressed (cross-CLI table cell
clarified; schedule-gating risk added to §8). Note 9
(`$USER` placeholder) confirmed correct, no change.

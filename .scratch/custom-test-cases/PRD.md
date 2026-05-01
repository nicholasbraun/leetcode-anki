# PRD: Custom Test Cases + better Run Result layout

## Problem Statement

When I'm working through a Problem and Submit a Solution, LeetCode sometimes comes back with Wrong Answer (or Runtime Error) and tells me which input I failed on. In LeetCode's web UI I can promote that failing input into the "Run" testcase panel with one click, then iterate locally — Run, look at the diff, fix, Run again — until that specific input passes. The TUI gives me no equivalent: I see the failing input on the Submit Result screen, but the next Run only feeds the Problem's Example Test Cases, so the failure I'm trying to chase isn't reproducible. My options today are to copy-paste through the browser or to embed the input as a hardcoded test in my Solution file. Both break the loop the TUI is supposed to support.

Two related papercuts on the Run Result screen compound the friction:

1. Per-case blocks stack vertically in a single column. Even three Example Test Cases overflow on a normal-sized terminal.
2. There's no scroll. Cases below the fold are simply unreachable.

## Solution

The TUI grows the concept of a **Custom Test Case**: a test input the user attaches to a Problem, persisted on disk, fed into every subsequent Run alongside the Problem's Example Test Cases.

When a Submit comes back with a non-Accepted Verdict and a usable failing input, the Submit Result screen shows a footer hint — `a  add to custom tests` — and pressing `a` appends that input to the Problem's Custom Test Case list. The next Run includes it. A `★` glyph next to a case index marks it as Custom so it's distinguishable from Examples. To curate the list, the Run Result screen accepts `x` followed by a digit to remove a Custom case by its displayed index (Examples can't be removed; pressing `x` on an Example index footer-warns and is a no-op).

The Run Result screen also gains a side-by-side grid layout — case blocks pack into 2-3 columns based on terminal width — and a body-only viewport so cases below the fold scroll into view without losing the verdict header, breadcrumb, or footer.

## User Stories

1. As a leetcode-anki user working through a Problem, I want a way to capture the failing input from a Wrong-Answer Submit so the next Run reproduces my failure locally.
2. As a leetcode-anki user, I want a single keystroke to promote that failing input — not a copy-paste workflow through the browser — so the TUI feels like a complete loop.
3. As a leetcode-anki user, I want the captured Custom Test Cases to survive TUI restarts so I don't have to re-discover the same edge cases tomorrow morning.
4. As a leetcode-anki user, I want Custom Test Cases scoped per-Problem (not per-language) so my edge cases follow the Problem when I switch from Go to Python.
5. As a leetcode-anki user, I want adding the same failing input twice to be a no-op so I don't accumulate duplicates from repeated failed Submits.
6. As a leetcode-anki user, I want a clear visual marker (`★`) on Custom cases in the Run Result screen so I know at a glance which inputs I added and which came from LeetCode.
7. As a leetcode-anki user, I want continuous numbering across Examples and Customs (1, 2, 3, ★4, ★5) so the keyboard removal command has one obvious referent.
8. As a leetcode-anki user, I want a way to remove a Custom Test Case I no longer need so my list doesn't accumulate stale inputs from problems I've since solved.
9. As a leetcode-anki user, I want removal to require an explicit two-step keystroke (`x` then a digit) so a stray keypress can't drop a case I spent time discovering.
10. As a leetcode-anki user, I want pressing `x` on an Example index to fail loudly with a footer message instead of silently doing nothing so I learn the rule.
11. As a leetcode-anki user, I want the Run Result screen to lay out per-case blocks side-by-side so three Example cases don't overflow my normal terminal.
12. As a leetcode-anki user, I want the case grid to adapt to my terminal width — 1 column on narrow terminals, 2-3 on wider ones — so the layout doesn't look broken at any size.
13. As a leetcode-anki user, I want the Run Result body to scroll when there are more cases than fit on screen so I can review every case without resizing my terminal.
14. As a leetcode-anki user, I want the verdict header (`✗ Wrong Answer`, `✓ Accepted`), breadcrumb, and footer to stay anchored when I scroll so I never lose context about where I am.
15. As a leetcode-anki user, I want scroll keys (`↑/↓`, `j/k`, `pgup/pgdn`) on the Run Result screen to match the keys I already use on the Problem description pane so I don't have to learn new bindings.
16. As a leetcode-anki user, I want the `a` key to be advertised in the footer only when there's actually something to add (a non-Accepted Submit with a non-empty `LastTestcase`) so the affordance doesn't lie about what's available.
17. As a leetcode-anki user, I want pressing `a` on an Accepted Submit (or a Compile Error with no `LastTestcase`) to be a no-op rather than save garbage so I can't break my Custom list with a misclick.
18. As a leetcode-anki user, I want my Custom Test Cases stored in plain JSON on disk so I can inspect or edit them with my editor when I need to.
19. As a leetcode-anki user, I want the Custom storage to be private (mode `0o600`) so my test inputs don't leak into world-readable cache files.
20. As a leetcode-anki user using Compile Error / Runtime Error verdicts, I want the same `a`-promotes flow as Wrong Answer so I can chase the input that crashed my Solution.
21. As a leetcode-anki user reading the project's domain documentation, I want `Custom Test Case` and `Example Test Case` to be canonical terms in `CONTEXT.md` so the codebase, tickets, and conversations stay aligned.
22. As a leetcode-anki user, I want the existing Run Result tests and behavior to keep working unchanged so the new layout and storage don't regress what I already use.
23. As a leetcode-anki user in Review Mode, I want Custom Test Cases to feed the Run on a Review attempt the same way they do in Explore Mode so my edge cases support recall practice too.
24. As a leetcode-anki contributor, I want the Custom storage layer to be testable in isolation (no TUI, no LeetCode, no real `$XDG_CACHE_HOME`) so storage bugs are caught locally without flaky integration tests.

## Implementation Decisions

### Domain language
- `Example Test Case` — read-only, supplied by `ProblemDetail.ExampleTestcases`.
- `Custom Test Case` — mutable user state, persisted per-Problem.
- A Run feeds Examples + Customs to `interpret_solution`, in that order, joined with `\n`.
- Both terms added to `CONTEXT.md` (already done during grilling). `Run` definition updated to mention both kinds.

### Storage
- New package: `internal/cases/`. Keeps it independent of `internal/editor` (which is Solution-specific) and `internal/leetcode` (which is wire-only).
- Interface (three methods):
  - `List(slug) ([]string, error)` — empty list on missing file; error on corrupt JSON.
  - `Add(slug, input) error` — silent dedupe.
  - `Remove(slug, index) error` — 0-based; out-of-range returns an error.
- On-disk file: `<UserCacheDir>/leetcode-anki/<slug>/custom_testcases.json`, mode `0o600`. Schema-versioned (`{"version": 1, "cases": [...]}`).
- Slug validation duplicates `editor.validSlug`'s regex locally so `cases` doesn't depend on `editor`.

### Domain interface (TUI)
- TUI gains a `Cases` interface alongside `LeetcodeClient`, `SolutionCache`, `Editor`. Production wires `cases.NewDiskCases()`; tests wire a `fakeCases`.
- `Model` gains a `cases Cases` field; `NewModel` takes it as a parameter; `cmd/leetcode-anki/main.go` constructs the disk impl.

### Run wiring
- `runCodeCmd` reads Customs via `Cases.List`, concatenates with `ExampleTestcases` (joined with `\n`), passes the combined string to `InterpretSolution`.
- A new helper `leetcode.CountCases(dataInput, metaData) int` is exported (extracted from existing logic in `run.go`). Used by `runCodeCmd` to compute `exampleCount` from the Problem's `ExampleTestcases` before merging.
- `runResultMsg` carries both the `*RunResult` and `exampleCount`. The result view stashes both.

### Result view rendering
- Per-case blocks render in an auto-fit grid: `cols = max(1, width / 38)`. Wraps to additional rows when more cases than fit per row.
- A renderer helper `renderCaseGrid(cases []RunCase, exampleCount int, width int) string` is extracted from `runBody` so it can be tested in pure string-in/string-out terms.
- `★` glyph (styled with `breadcrumbActiveStyle`) prefixes the verdict on cases at index ≥ `exampleCount`. Continuous numbering across both kinds.
- Body-only `viewport.Model` in `resultView`. Verdict header, breadcrumb, top/bottom dividers, and footer stay outside the viewport. Sized from `m.width` / `m.height` minus chrome.
- Unhandled key messages fall through to `viewport.Update`, matching the pattern used in `problem_view.go`. Screen-action keys (`a`, `x`, digit-after-`x`, `Back`, `Enter`) are handled before fallthrough so they don't get eaten by the viewport.

### `a` key (Submit Result, promote LastTestcase)
- Available only when `m.result.kind == resultSubmit && m.result.submit != nil && m.result.submit.LastTestcase != ""`.
- Footer entry `a  add to custom tests` shown conditionally on the same predicate.
- On press: `cases.Add(slug, m.result.submit.LastTestcase)`. Errors set `m.err`. A short toast above the footer confirms success on the next render.

### `x` key (Run Result, remove Custom)
- `resultView` gains `awaitingRemoveDigit bool`. `x` while a Run is being viewed and `len(cases) > 0` sets the flag and changes the footer hint to `1-9  remove case`.
- Next digit `1`–`9` (1-based, mapped to 0-based) chooses the case. If `< exampleCount`: footer toast "case N is an Example — can't remove" and clear flag. Otherwise: `cases.Remove(slug, i - exampleCount)` and clear flag.
- `esc` while flag is set: clear flag, no removal.
- `x` cap at digit 9 is a v1 limitation. Documented in code comment.

## Testing Decisions

A good test in this codebase exercises *external behavior*, not implementation details. The existing TUI tests (e.g. `result_view_test.go`, `edit_flow_test.go`) drive `m.Update(tea.KeyMsg{...})` and assert against rendered output (`strings.Contains` on `viewResultView(m)`) or against fakes that record their calls. Storage tests do round-trip writes/reads against `t.TempDir()`. We follow both patterns directly.

### Modules with new tests

- **`internal/cases`** (round-trip + edge cases):
  - Add → List returns the input.
  - Add same input twice → List returns one entry (dedupe).
  - Remove valid index → List returns the remainder; out-of-range → error.
  - Missing file → List returns empty slice, no error.
  - Corrupt JSON → List returns an error; the file is *not* overwritten.
  - File mode `0o600` after first Add.
  - Invalid slug → all methods reject.
- **`internal/leetcode.CountCases`** (combinatorics):
  - 3 cases × 2 params × 6 lines → 3.
  - Empty input → 0.
  - Missing/malformed `metaData` → degrades gracefully (per existing fallback logic).
- **`internal/tui` case-grid renderer** (`renderCaseGrid`):
  - Width 130, three cases → all fit on one row (column count = 3).
  - Width 80, three cases → two rows (column count = 2).
  - Width 50, three cases → three rows (column count = 1).
  - `★` appears only on cases at index ≥ `exampleCount`.
  - 0 cases → empty string.
- **`internal/tui` Run wiring** (`runCodeCmd`):
  - `Cases.List` is consulted for the Problem's slug.
  - The `dataInput` passed to `InterpretSolution` (captured via `leetcodefake.RunHook`) equals `ExampleTestcases + "\n" + customCase`.
  - `runResultMsg` carries `exampleCount` matching the Problem's Example count.
- **`internal/tui` Submit `a` key**:
  - Wrong Answer with non-empty `LastTestcase` + `a` → `Cases.Add` called with `(slug, LastTestcase)`.
  - Accepted Submit + `a` → no `Cases.Add` call.
  - Compile Error with empty `LastTestcase` + `a` → no `Cases.Add` call.
- **`internal/tui` Run `x`-then-digit**:
  - 2 Examples + 1 Custom + `x` `3` → `Cases.Remove(slug, 0)`.
  - Same setup + `x` `1` → no Remove (Example), footer warns.
  - `x` then `esc` → no Remove, flag cleared.
  - `x` with no cases → no-op (flag stays false).
- **`internal/tui` viewport scroll**:
  - Tall result body + `pgdn` → `bodyVP.YOffset` increases.
  - Verdict header substring still present in the rendered output after scroll (anchor proof).

### Prior art for tests
- `internal/tui/result_view_test.go` — driving `Update` with key messages, asserting via `strings.Contains` on rendered output.
- `internal/tui/edit_flow_test.go` — staging a `*ProblemDetail`, exercising the full Edit→Run flow with `leetcodefake.Fake`.
- `internal/leetcode/run_test.go` — table-driven tests for `splitDataInput` / `metaParamCount`. New `CountCases` tests should follow the same shape.
- `internal/sr/cache_test.go` — disk-backed JSON storage round-trip tests against `t.TempDir()`. New `cases` tests should follow the same shape (path injection or `XDG_CACHE_HOME` override).
- `internal/tui/cancel_flow_test.go` — pattern for hooks that capture the args a fake was called with (used to assert `dataInput` content in the Run wiring tests).

## Out of Scope

- **Manual entry of a Custom Test Case** (typing a free-form test input into the TUI). Useful follow-on, but it has its own input/validation/UX surface and can be added behind a separate key without redoing v1.
- **Promoting Run failures to Custom**. A Run-failed case is already in Examples or Customs by definition; promotion would always be a no-op or a dedupe.
- **A dedicated "manage Custom Test Cases" screen**. Removal lives on the Run Result screen via `x`+digit; a list-management screen is overkill for v1.
- **Auto-removal on Accepted Verdict**. Tempting but surprising — Accepted means the Solution survived the *full grader battery*, not specifically the Custom case. Custom cases stay until the user removes them.
- **Removing more than 9 Custom cases via the keyboard**. Cases at index 10+ require filesystem cleanup. v1 cap; revisit if real usage hits it.
- **Editing an existing Custom Test Case in place**. Workflow is "remove + re-add."
- **Live-contract test coverage** for this flow. Custom storage is local-only; the `interpret_solution` call already accepts arbitrary `dataInput` and is exercised by the existing live contract.

## Further Notes

- `LastTestcase` from a non-Accepted Submit is observed to come back as a single test input in the same multi-line param-count format as one Example case. So appending it to `ExampleTestcases` with a `\n` separator produces a `data_input` that `splitDataInput` parses correctly. This is the load-bearing assumption; the `internal/tui` Run wiring tests pin it.
- The new `Cases` interface should be wired through `NewModel`. Don't fall back to a package-level singleton — that would defeat the test-injection pattern that the rest of the TUI follows.
- The `★` glyph choice is intentionally subtle (no extra hue beyond `breadcrumbActiveStyle`'s sky blue). The Result screen is already visually busy; one extra distinguishing mark is enough.
- v1 ships with a six-commit sequence: layout → viewport → storage → wire+glyph → `a` key → `x`+digit. Each commit is independently shippable and atomic per `CLAUDE.md`. See `.claude/plans/snuggly-wibbling-chipmunk.md` for the per-commit detail.

## Comments

> *This was generated by AI during triage.*

Parent spec — no triage state role. Actionable work lives on the child issues under `issues/`, all currently `ready-for-agent`:

- [01-run-case-grid-layout.md](issues/01-run-case-grid-layout.md)
- [02-scrollable-result-body.md](issues/02-scrollable-result-body.md)
- [03-custom-test-cases-add-and-render.md](issues/03-custom-test-cases-add-and-render.md)
- [04-remove-custom-test-case.md](issues/04-remove-custom-test-case.md)

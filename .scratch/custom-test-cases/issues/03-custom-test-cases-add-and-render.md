# Custom Test Cases: add from Submit-fail and render in Run

Status: shipped (commit 3f010b5)

## Parent

[../PRD.md](../PRD.md)

## What to build

Wire the full Custom Test Case flow end-to-end: storage layer, Run wiring, glyph rendering, and the `a` key on Submit Result that promotes a failing input. None of these subparts are demoable on their own — this is the unavoidable tracer-bullet slice for the feature.

### Storage layer

- New package `internal/cases/`. Independent of `internal/editor` (which is Solution-specific) and `internal/leetcode` (which is wire-only).
- Public interface:
  ```
  type Cases interface {
      List(slug string) ([]string, error)
      Add(slug, input string) error      // silent dedupe — adding an existing case is a no-op
      Remove(slug string, index int) error // 0-based; out-of-range returns error
  }
  ```
- `DiskCases` implementation backed by `<UserCacheDir>/leetcode-anki/<slug>/custom_testcases.json`, mode `0o600`, schema-versioned (`{"version": 1, "cases": [...]}`).
- Missing file → empty list, no error. Corrupt JSON → return error; do **not** overwrite.
- Slug validation: duplicate `editor.validSlug`'s regex locally so `cases` does not depend on `editor`.

### Run wiring

- Export `leetcode.CountCases(dataInput, metaData string) int` — a small helper extracted from existing logic in `internal/leetcode/run.go` (uses `metaParamCount` + line counting). Returns 0 when input is empty or undividable.
- TUI gains a `Cases` interface in `internal/tui/client.go` alongside `LeetcodeClient`/`SolutionCache`/`Editor`. Production wires `cases.NewDiskCases()`; tests wire a `fakeCases` in `internal/tui/helpers_test.go`.
- `Model` gains a `cases Cases` field; `NewModel` takes it as a parameter; `cmd/leetcode-anki/main.go` constructs `cases.NewDiskCases()` and passes it through.
- `runCodeCmd` in `internal/tui/app.go`:
  - Calls `cases.List(slug)` before `InterpretSolution`.
  - Builds `dataInput = ExampleTestcases + "\n" + strings.Join(customs, "\n")` (skip the leading `\n` if either is empty).
  - Calls `InterpretSolution` with the combined string.
  - Returns a `runResultMsg` carrying both `*RunResult` and `exampleCount` (computed via `leetcode.CountCases(p.ExampleTestcases, p.MetaData)`).
- `resultView` gains an `exampleCount int` field, set when a `runResultMsg` arrives.

### Glyph rendering

- The `renderCaseGrid` helper introduced in #01 now uses `exampleCount`: any case at index `>= exampleCount` is a Custom case and gets a `★` glyph between its index and verdict (`case 4 ★ ✓ pass`).
- Glyph styled with `breadcrumbActiveStyle` from `styles.go`.
- Numbering is continuous across Examples and Customs.
- The footer on a Run Result with at least one Custom case includes the legend `★  custom`.

### `a` key (Submit Result)

- In `viewResultView`, when `m.result.kind == resultSubmit && m.result.submit != nil && m.result.submit.LastTestcase != ""`, append `footerItem{"a", "add to custom tests"}` to the footer.
- In `updateResultView`, on `a` with the same gate, call `cases.Add(m.currentProblem.TitleSlug, m.result.submit.LastTestcase)`.
- Errors set `m.err`. On success, set a transient `m.result.toast = "added"` rendered above the footer for one redraw, cleared on the next key.

### Domain documentation

- `CONTEXT.md` already updated during grilling (Example Test Case, Custom Test Case, expanded Run definition, new relationship line).
- This slice does not introduce further domain language.

## Acceptance criteria

### Storage

- [ ] `Add` then `List` returns `[input]`.
- [ ] `Add` same input twice → `List` returns one entry (silent dedupe).
- [ ] `Remove` valid index → `List` returns the remainder.
- [ ] `Remove` out-of-range index → returns an error.
- [ ] Missing file → `List` returns an empty slice with no error.
- [ ] Corrupt JSON file → `List` returns an error; the file is **not** overwritten by subsequent `Add` calls until the user fixes it. (Acceptable refinement: subsequent `Add` returns the corrupt-JSON error too.)
- [ ] After first `Add`, the file mode is `0o600`.
- [ ] All methods reject invalid slugs (paths with `..`, slashes, etc.).

### Helper

- [ ] `leetcode.CountCases("a\nb\nc\nd", metaDataWith2Params) == 2`.
- [ ] `leetcode.CountCases("", anything) == 0`.
- [ ] Malformed `metaData` falls back to per-line behavior gracefully (matches existing `splitDataInput` fallback semantics).

### Run wiring

- [ ] With Examples `"[1,2]\n3"` (1 case, 2 params) and Custom `"[4,5]\n9"`, `runCodeCmd` calls `InterpretSolution` with `dataInput == "[1,2]\n3\n[4,5]\n9"`. Captured via `leetcodefake.RunHook`.
- [ ] `runResultMsg.exampleCount` equals the Problem's Example count after the call.
- [ ] When the user has no Custom cases on a Problem, `dataInput == p.ExampleTestcases` (no trailing newline introduced).

### Rendering

- [ ] Run Result on a Problem with 2 Examples + 1 Custom shows `★` only on case 3 in the rendered output.
- [ ] When at least one Custom case is present, the footer legend `★  custom` is rendered.

### `a` key

- [ ] Wrong Answer + non-empty `LastTestcase` + `a` → `Cases.Add` called once with `(slug, LastTestcase)`.
- [ ] Footer hint `a  add to custom tests` is present on Wrong Answer / Runtime Error with non-empty `LastTestcase`.
- [ ] Accepted Submit + `a` → no `Cases.Add` call; no footer hint.
- [ ] Compile Error with empty `LastTestcase` + `a` → no `Cases.Add` call; no footer hint.
- [ ] `Cases.Add` returning an error sets `m.err`.
- [ ] Successful add renders a brief toast `"added"` near the footer.

### Quality gates

- [ ] `go vet ./... && go build ./... && go test ./...` all pass before commit.
- [ ] All commits in this slice are individually atomic — `vet`, `build`, and `test` pass at every commit, not only at the end.

## Blocked by

None — can start immediately. (Visually pairs better with #01 having landed first since the grid renderer is where the `★` glyph lives, but storage + wiring + `a` key can ship without the grid.)

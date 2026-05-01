# Run case grid layout (side-by-side)

Status: shipped (commit 3ebbbee)

## Parent

[../PRD.md](../PRD.md)

## What to build

Replace the single-column stack of per-case blocks in the Run Result screen with an auto-fit grid. Per-case blocks render side-by-side in 1–3 columns based on terminal width:

- `cols = max(1, width / 38)` — 38-char minimum per column.
- Inside each column, the labeled rows (`input`, `your output`, `expected`, `stdout`, pass/fail header) stack vertically as today. Only the cases relative to each other go horizontal.
- When more cases than fit in one row, wrap to the next row.

Extract a `renderCaseGrid(cases []RunCase, exampleCount int, width int) string` helper from the existing `runBody` so the layout logic can be unit-tested in pure string-in/string-out form. The `exampleCount` parameter is plumbed through but unused in this slice (no glyph yet — that ships in #03); the parameter is in place so the helper signature is stable.

## Acceptance criteria

- [x] 3 cases at width 130 render in 1 row of 3 columns (case 1 │ case 2 │ case 3).
- [x] 3 cases at width 80 render in 2 rows (case 1 │ case 2 / case 3).
- [x] 3 cases at width 50 fall back to 1-per-row (case 1 / case 2 / case 3) — preserves today's behavior on narrow terminals.
- [x] Existing per-case block contents (input, output, expected, stdout, pass/fail header) preserved verbatim per column.
- [x] All existing tests in `internal/tui/result_view_test.go` still pass.
- [x] New tests for `renderCaseGrid` cover the three width breakpoints and the empty-cases case.
- [x] `go vet ./... && go build ./... && go test ./...` all pass before commit.

## Blocked by

None — can start immediately.

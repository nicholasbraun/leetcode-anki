# Remove a Custom Test Case via `x`+digit

Status: shipped (commit 69ae066)

## Parent

[../PRD.md](../PRD.md)

## What to build

Curate the Custom Test Case list from inside the TUI: on the Run Result screen, `x` followed by a digit `1`–`9` removes the Custom case at that displayed index. Two-step keystroke prevents accidental deletion.

- `resultView` gains `awaitingRemoveDigit bool`.
- On the Run Result screen with `len(cases) > 0`:
  - `x` → set `awaitingRemoveDigit = true`. Footer hint switches to `1-9  remove case · esc  cancel`.
  - Any digit `1`–`9` while flag is set: map to 0-based case index `i = digit - 1`. Then:
    - If `i < exampleCount` → footer toast `"case N is an Example — can't remove"`, clear flag, no removal.
    - If `i >= exampleCount` → call `cases.Remove(slug, i - exampleCount)`. Clear flag.
    - If `i >= len(cases)` → no-op, clear flag.
  - `esc` → clear flag, no removal.
  - Any other key while flag is set → clear flag, route the key normally (so e.g. arrows still scroll the viewport).
- On the Run Result screen with no cases, `x` is a no-op (flag stays false).
- After a successful `Cases.Remove`, the result view's `Cases` slice is **not** updated retroactively (it still shows the previous Run's verdicts). The removal takes effect on the next Run. This avoids re-rendering with stale indices.
- v1 limitation: cases at displayed index 10+ cannot be removed via the keyboard (single-digit only). Documented as a code comment.

## Acceptance criteria

- [ ] With 2 Examples + 1 Custom, `x` then `3` calls `Cases.Remove(slug, 0)` exactly once.
- [ ] With the same setup, `x` then `1` does **not** call `Cases.Remove`; the footer renders the "case 1 is an Example — can't remove" toast.
- [ ] `x` then `esc` clears the flag without calling `Cases.Remove`.
- [ ] `x` while there are no cases on the Run Result screen does not set the flag (and the footer hint does not change).
- [ ] While `awaitingRemoveDigit` is true, the footer shows `1-9  remove case · esc  cancel`.
- [ ] After any digit press (valid or not), the flag is cleared.
- [ ] `Cases.Remove` returning an error sets `m.err`.
- [ ] `go vet ./... && go build ./... && go test ./...` all pass before commit.

## Blocked by

- Blocked by [./03-custom-test-cases-add-and-render.md](./03-custom-test-cases-add-and-render.md) — depends on Custom Test Cases existing in the model and on the `Cases.Remove` storage method being available.

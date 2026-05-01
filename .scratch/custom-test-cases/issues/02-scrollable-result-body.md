# Scrollable Run Result body

Status: shipped (commit 768869f)

## Parent

[../PRD.md](../PRD.md)

## What to build

Add a body-only `viewport.Model` to `resultView` so long Run/Submit bodies (many cases, long error traces) scroll without the user losing the verdict header or footer.

- The viewport wraps only the body string. The breadcrumb, top divider, verdict header (`✗ Wrong Answer`, `✓ Accepted`), bottom divider, and footer stay outside it and remain anchored.
- Sized in `viewResultView` from `m.width`/`m.height` minus the chrome height.
- `updateResultView` falls through unhandled key messages to `m.result.bodyVP, cmd = m.result.bodyVP.Update(msg)` — same pattern as `problemView` does for the description pane.
- `Back`/`Enter` are handled before the fallthrough so they exit the screen and don't get eaten by the viewport.
- Reuses viewport's default keys: `↑/↓`, `j/k`, `pgup/pgdn`, `home/end`. No new bindings.

This applies to both Run and Submit result bodies — the same viewport wraps either kind. Compile/runtime error traces benefit from the scroll too.

## Acceptance criteria

- [ ] A Run Result with enough cases that the body exceeds the visible height does not overflow the screen; instead the viewport clips, and scroll keys advance through the body.
- [ ] After scrolling, the verdict header substring (e.g. `"✗ Wrong Answer"`) is still present in the rendered output (anchor proof).
- [ ] After scrolling, the footer is still present in the rendered output.
- [ ] `Back` (esc) and `Enter` still exit the result screen — viewport does not eat them.
- [ ] New test: a tall result body + `tea.KeyMsg{Type: tea.KeyPgDown}` advances `bodyVP.YOffset` from 0 to a positive value.
- [ ] New test: after scrolling, `viewResultView(m)` still contains the header/footer substrings.
- [ ] `go vet ./... && go build ./... && go test ./...` all pass before commit.

## Blocked by

None — can start immediately. (Independent of #01; both touch `result_view.go` so doing them in series avoids merge conflicts, but neither is a hard prereq for the other.)

# Quota-aware empty state + budget-aware footer

Status: needs-triage

## Parent

[../PRD.md](../PRD.md)

## What to build

After #02 lands the queue stays empty post-quota, but the empty pane still looks identical to a list with no due-or-new work — exactly the ambiguity that prompted the original bug report. This slice closes that loop with two visible affordances on the Problems screen, both gated on `m.reviewMode == true && m.session != nil`:

1. The empty-state copy branches on `Session.QuotaExhausted`. When `true`, the message names the cause and points at the local-midnight rollover. When `false`, the existing empty-pane copy is preserved.
2. A footer below the queue shows budget consumption: `<reviewsDone> of <reviewQuota> reviews done today · <newDone> of <newQuota> new`. Visible whether the queue is empty or populated, so the user can see how the budget stands at any point during a session.

The footer string updates as the user AC's items in the same TUI session — no restart required. This works because each Submit triggers `MarkAccepted` (#01), which invalidates the cache; the next `loadProblemsCmd` rebuilds the `Session` with fresh counters.

### Empty-state branch

- New helper in `internal/tui/problems_view.go` (or co-located): `quotaExhaustedMessage()` returns the user-facing string for the exhausted state. Kept as a pure function so the test suite can assert exact copy without driving the full TUI.
- The empty-pane render path in the Problems-list view checks `session != nil && session.QuotaExhausted` and picks the exhausted message; otherwise falls through to the existing empty pane (no copy change to the genuinely-empty case).
- Copy: "you've reviewed your daily quota — next batch unlocks at midnight" (or a close variant — final wording at the implementer's discretion, but it must name "quota," and reference "midnight" or "tomorrow").

### Budget-aware footer

- New helper: `quotaFooter(session *sr.Session, cfg quotaCfg)` returns the rendered string. Takes the bucket sizes (`reviewQuota`, `newQuota` from the model — equivalent to `m.reviewDue` / `m.reviewNew`) and the consumed-today counters from `session`.
- Format: `<reviewsDone> of <reviewQuota> reviews done today · <newDone> of <newQuota> new`. When a bucket size is zero (user disabled it via `--review-due 0`), that bucket is omitted from the footer string entirely (no `0 of 0` clutter).
- Rendered below the queue in the Problems-screen view, in the same vertical region as the existing breadcrumb / footer hints. Style: dim, consistent with the existing footer affordances.
- Visible only when `m.reviewMode == true && m.session != nil`; Explore Mode is unchanged.

### Live updates

- The footer string is computed fresh on every render (it reads from `m.session`, which `loadProblemsCmd` rebuilds after each successful AC). No new state on the model.
- After AC + grade-or-esc, the existing flow already calls `loadProblemsCmd` (via `advanceToNextDue` or equivalent); the new `Session` carries the updated counters, and the footer reflects them on the next render.

### Domain documentation

- No `CONTEXT.md` changes — Quota Exhausted, Review Quota, New Quota are already canonical from the grilling pass.

## Acceptance criteria

### Empty-state branch

- [ ] `Session.QuotaExhausted == true` and `len(session.Items) == 0` → the empty pane renders the quota-exhausted message (asserted via substring match on "quota" and "midnight").
- [ ] `Session.QuotaExhausted == false` and `len(session.Items) == 0` → the existing empty pane is rendered (no quota copy).
- [ ] Explore Mode (`reviewMode == false`) → empty pane is unchanged regardless of `Session` content.
- [ ] `m.session == nil` (degraded SR load) → empty pane is unchanged (no panic, no quota copy).

### Footer

- [ ] In Review Mode with `Session.ReviewsDoneToday=1`, `MaxDue=2`, `NewDoneToday=0`, `MaxNew=1` → footer renders `1 of 2 reviews done today · 0 of 1 new`.
- [ ] After AC'ing one item in the same session and re-rendering, the footer increments to reflect the new `ReviewsDoneToday`.
- [ ] `MaxDue=0` (review bucket disabled) → footer omits the reviews segment, renders only the new segment.
- [ ] `MaxNew=0` (new bucket disabled) → footer omits the new segment, renders only the reviews segment.
- [ ] `MaxDue=0` and `MaxNew=0` → footer is empty / not rendered.
- [ ] Explore Mode → footer is not rendered (no quota concept applies).
- [ ] `m.session == nil` → footer is not rendered (no panic).

### Quality gates

- [ ] `go vet ./... && go build ./... && go test ./...` all pass before commit.
- [ ] All commits in this slice are individually atomic — `vet`, `build`, and `test` pass at every commit, not only at the end.
- [ ] Empty-state and footer tests follow the existing `internal/tui/problems_view_test.go` style — render with a hand-built `*sr.Session` and assert on the rendered string.
- [ ] At least one test exercises the full AC → re-render → updated footer path through the TUI, in the style of `internal/tui/review_test.go` / `internal/tui/review_edit_flow_test.go`.
- [ ] New tests follow the TDD red-green-refactor loop per the project convention.

## Blocked by

- Blocked by `02-daily-quota-math-and-session.md`. The empty-state branch and footer both read `Session.QuotaExhausted` / `ReviewsDoneToday` / `NewDoneToday`, which #02 introduces.

# Daily Quota math + Session quota awareness

Status: needs-triage

## Parent

[../PRD.md](../PRD.md)

## What to build

This is the slice that fixes the original bug. After this lands, AC'ing today's Review Mode queue and restarting the TUI keeps the queue empty until local midnight, per Problem List, regardless of how many times the TUI is reopened.

A new pure quota-math function inside `internal/sr` derives `(reviewsDoneToday, newDoneToday)` from cached submissions for a list's slugs. `Reviews.Session()` consumes those counters, applies them as a per-day budget on top of `cfg.MaxDue` / `cfg.MaxNew`, and exposes the new state through `Session.QuotaExhausted`, `Session.ReviewsDoneToday`, and `Session.NewDoneToday` so the view layer can render quota-aware copy without re-deriving the math.

This slice does not change the empty-state copy or footer rendering — the new fields land but the Problems screen still uses the existing empty pane. The visible behavior change is that the queue stays empty after AC; the UX polish is #03.

### Quota math

- New file `internal/sr/quota.go`. New function:
  ```
  func consumedToday(
      slugs []string,
      cache map[string]slugEntry,
      sched scheduler,
      now time.Time,
  ) (reviewsDone, newDone int)
  ```
- For each slug in `slugs`:
  - Walk the slug's cached submissions in chronological order.
  - "Today" is defined as `[startOfDay(now, time.Local), now]` inclusive of the lower bound.
  - Find the first submission whose `OccurredAt` falls in that window AND `Accepted == true`.
    - If no prior Accepted submission exists for the slug → it's a "new done today": `newDone++`. (Subsequent ACs same day on the same slug do not double-count.)
    - Else, compute the SM-2 schedule over all prior submissions (those before the today-AC) via the passed-in `scheduler`. If `sched.schedule(prior reviews) ≤ at-time-of-AC`, the slug was due → `reviewsDone++`. Otherwise the AC was voluntary practice → not counted.
- Wrong-Answer Submits, ACs from yesterday, and second-and-later ACs on the same slug same day all count for nothing.
- Pure: no I/O, deterministic, idempotent.

### Session integration

- `internal/sr/reviews.go` `Session()`:
  - After fetching UserProgress and resolving the cache (existing flow), call `consumedToday(cfg.Slugs, r.cache.Slugs, r.sched, now)`.
  - Effective bucket budgets: `effDue = max(0, cfg.MaxDue - reviewsDone)`, `effNew = max(0, cfg.MaxNew - newDone)`.
  - The walk over `cfg.Slugs` keeps its current "due first, then new" ordering. Caps swap from `cfg.MaxDue` / `cfg.MaxNew` to `effDue` / `effNew`.
  - `DueTotal` / `NewTotal` continue to report uncapped counts (unchanged).
  - Set `Session.ReviewsDoneToday = reviewsDone`, `Session.NewDoneToday = newDone`.
  - Set `Session.QuotaExhausted` per definition below.
- New `Session` fields:
  ```
  QuotaExhausted   bool // true when the queue is empty because budgets are zero
  ReviewsDoneToday int  // count of Reviews already AC'd today on slugs in cfg.Slugs
  NewDoneToday     int  // count of first-ever ACs already done today on slugs in cfg.Slugs
  ```
- `QuotaExhausted` definition: true iff `len(Items) == 0` AND at least one slug in `cfg.Slugs` would have qualified for either bucket, but the effective budget for that bucket was zero. False when `len(Items) == 0` because the list genuinely had no due-or-new candidates (nothing to exhaust).

### CLI / config

- `cmd/leetcode-anki/main.go`: update help text on `--review-due` and `--review-new` from "per-Session cap" wording to "per-day quota size for the Review / New bucket." Flag names unchanged.
- `defaultReviewDue = 2` and `defaultReviewNew = 1` constants unchanged.

### Domain documentation

- `CONTEXT.md` was updated during grilling (Daily Quota / Review Quota / New Quota / Quota Exhausted, plus relationship lines and ambiguity flag). No further changes here.
- ADR-0001 (`docs/adr/0001-daily-quota-for-review-mode.md`) is the design-rationale record.

## Acceptance criteria

### Quota math (pure function)

- [ ] No submissions for any slug → `(0, 0)`.
- [ ] One AC today on a slug with a prior AC, where SM-2 schedule says the slug was due → `(1, 0)`.
- [ ] One AC today on a slug with a prior AC, where SM-2 schedule says the slug was NOT yet due (voluntary practice) → `(0, 0)`.
- [ ] First-ever AC today on a slug → `(0, 1)`.
- [ ] AC at exactly `startOfDay(now, time.Local)` (boundary inclusion) → counted.
- [ ] AC one second before `startOfDay(now, time.Local)` → not counted.
- [ ] Two ACs on the same slug same day, both after a prior-day AC → `(1, 0)` (first counts, second doesn't double-count).
- [ ] Wrong-Answer Submit today on a tracked-and-due slug → `(0, 0)`.
- [ ] AC from yesterday → `(0, 0)`.
- [ ] Slug not in the cache map → contributes nothing (no panic).
- [ ] Multiple slugs each contributing a different category aggregate correctly: e.g. one due-AC + one new-AC + one voluntary-practice + one yesterday-AC → `(1, 1)`.

### Session integration

- [ ] List has 5 due, `MaxDue=2`, no AC today → `Items` carries 2 KindDue, `ReviewsDoneToday=0`, `QuotaExhausted=false`, `DueTotal=5`.
- [ ] List has 5 due, `MaxDue=2`, 1 due AC'd today → `Items` carries 1 KindDue (the next still-due slug), `ReviewsDoneToday=1`, `QuotaExhausted=false`, `DueTotal=4` (the still-due count).
- [ ] List has 5 due, `MaxDue=2`, 2 due AC'd today → `Items` empty, `ReviewsDoneToday=2`, `QuotaExhausted=true`, `DueTotal=3`.
- [ ] List has 5 due, `MaxDue=2`, 3 due AC'd today (e.g. via Explore Mode beyond the cap) → `Items` empty, `ReviewsDoneToday=3`, `QuotaExhausted=true`, effective budget clamped at 0.
- [ ] List has 0 due and 0 new → `Items` empty, `QuotaExhausted=false` (genuinely empty, nothing to exhaust).
- [ ] List has new candidates, `MaxNew=1`, 1 new AC'd today → no KindNew item surfaces, `NewDoneToday=1`, `QuotaExhausted=true` (when due bucket is also empty/exhausted).
- [ ] `MaxDue=0` (bucket disabled) → no KindDue items; `ReviewsDoneToday` still computed for accounting.
- [ ] Existing per-bucket cap behavior is preserved when `reviewsDoneToday=0` and `newDoneToday=0` (no regression).

### TUI integration

- [ ] After AC'ing the queue and "restarting" (re-running `loadProblemsCmd` in test), `visibleProblems` returns an empty list and `Session.QuotaExhausted == true`.
- [ ] Within the same TUI session, AC'ing items decrements the effective budget on the next `loadProblemsCmd` invocation (counters reflect the cache after `MarkAccepted` from #01).
- [ ] No change to Explore Mode behavior — quota math is only invoked when `reviewMode == true`.

### CLI

- [ ] `--review-due --help` text mentions "per-day" semantics. (Same for `--review-new`.)

### Quality gates

- [ ] `go vet ./... && go build ./... && go test ./...` all pass before commit.
- [ ] All commits in this slice are individually atomic — `vet`, `build`, and `test` pass at every commit, not only at the end.
- [ ] Quota math has table tests in the style of `internal/sr/sm2_test.go`.
- [ ] Session integration tests follow the existing fake-`SubmissionsClient` pattern in `internal/sr/reviews_test.go`.
- [ ] New tests follow the TDD red-green-refactor loop per the project convention.

## Blocked by

- Blocked by `01-cache-invalidate-on-ac.md`. Without #01 the SR cache misses today's ACs whenever the user esc's out of the rating modal, and the quota math under-counts `reviewsDoneToday` / `newDoneToday`. Shipping #02 alone would deliver a visibly buggy esc-out path.

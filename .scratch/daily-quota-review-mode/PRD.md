# PRD: Daily Quota for Review Mode

Status: needs-triage

## Problem Statement

Review Mode is supposed to be the answer to "what should I work on today to keep my recall sharp." Today, when I open the TUI in Review Mode I see a small queue (default: 2 due Problems + 1 new) and I work through it. When I finish it, the queue empties — exactly what I'd expect for a daily routine.

But if I close the TUI and reopen it on the same day, the queue refills. Two different due Problems and a different new one appear, and if I solve those, I can keep going indefinitely. The "I'm done for today" signal is a lie: it's actually "I'm done with this Session," and a fresh Session is one cmd-tab away. The cap defends against doing too much in one sitting, but it doesn't defend against doing too much in one day, which is the actual goal of a spaced-repetition system.

The bug is that `--review-due` / `--review-new` are caps on a Session, not budgets for a day. Every time `loadProblemsCmd` runs, it builds a fresh Session by asking SR which slugs are still due and taking the first N — so as long as the list has more due-or-new candidates than the cap, restarts surface the next batch.

## Solution

Promote the caps from per-Session to per-day. Once I've burned my 2 reviews + 1 new for a Problem List today, that list's Review Mode queue stays empty until local midnight, regardless of how many TUI restarts I do. Each Problem List has its own daily budget — clearing List A's quota doesn't lock me out of List B.

The empty queue stops being one undifferentiated state. When the queue is empty because I'm done for the day, the Problems screen says so and tells me when the next batch unlocks. When it's empty because the list genuinely has no due work, it says that instead. A footer below the list shows "1 of 2 reviews done today · 0 of 1 new" so I can see how my budget stands at a glance, even mid-session.

The new domain language is **Daily Quota** (the per-list, per-day budget) split into a **Review Quota** (default 2) and a **New Quota** (default 1). When both are zero and the list still has due-or-new work that didn't fit, the list is **Quota Exhausted**. CONTEXT.md has been updated; ADR-0001 records the choice.

A latent bug ships alongside: the SR cache only invalidates a slug when the user grades a Submit, so esc'ing the rating modal currently leaves the cache permanently stale. Under the daily-quota design that becomes user-visible (an esc'd review wouldn't burn quota and would re-queue tomorrow). Cache invalidation moves to fire on every Accepted Submit, ahead of the rating modal.

## User Stories

1. As a leetcode-anki user, I want my Review Mode quota to be a per-day budget so closing and reopening the TUI on the same day doesn't refill the queue with another 2 reviews + 1 new.
2. As a leetcode-anki user, I want the quota to reset at local midnight so a new day reliably gives me my daily reviews regardless of when I last ran the TUI.
3. As a leetcode-anki user, I want each Problem List to have its own daily quota so finishing one list's reviews doesn't lock out a different list I'm also working through.
4. As a leetcode-anki user, I want any Accepted Submit today to count toward today's quota, regardless of whether I AC'd via Review Mode or Explore Mode, so the cap actually models how much SR work I've done.
5. As a leetcode-anki user, I want voluntary re-attempts of Problems that aren't yet due to NOT consume my Review Quota so practicing extra doesn't punish me tomorrow.
6. As a leetcode-anki user, I want the first-ever Accepted Submit on a Problem to consume my New Quota — and only the New Quota — so the daily cap on new introductions is respected.
7. As a leetcode-anki user, I want Wrong-Answer Submits to NOT consume any quota so failing a review doesn't burn my budget.
8. As a leetcode-anki user, I want the quota math to read from the existing SR submission cache so the feature doesn't add new on-disk state for something derivable.
9. As a leetcode-anki user, I want partial completion to behave naturally — solving 1 of 2 due, then closing and reopening, should still surface 1 due + 1 new — so the cap doesn't penalize me for not finishing in one sitting.
10. As a leetcode-anki user, I want raising `--review-due` mid-day to top up my queue with the difference (cap minus already-done) so I can flex the budget when I have more time.
11. As a leetcode-anki user, I want lowering `--review-due` below my already-done count to clamp the remainder to zero so I can't trick myself into more work by adjusting flags.
12. As a leetcode-anki user, I want a quota-exhausted Problem List to show a clear message ("you've reviewed your daily quota — next batch unlocks at midnight") so I know I'm done for the day rather than worrying the system is broken.
13. As a leetcode-anki user, I want a Problem List with no due-or-new work at all to show a different message ("nothing due in this list") so I can tell genuine emptiness from quota exhaustion at a glance.
14. As a leetcode-anki user, I want a footer below the Review Mode queue that shows my consumption against the daily quota (e.g. "1 of 2 reviews done today · 0 of 1 new") so I always know how the budget stands.
15. As a leetcode-anki user, I want the footer to keep working as I AC items in the same session so the counter ticks up without requiring a restart.
16. As a leetcode-anki user who esc's out of the rating modal after AC, I want my AC to still burn quota so my queue reflects work I actually did.
17. As a leetcode-anki user who esc's out of the rating modal, I want the SR scheduler to still see today's AC so I'm not stuck reviewing the same Problem on the next refresh because the cache went stale.
18. As a leetcode-anki user who AC's in Explore Mode while a Problem is also due, I want that AC to count toward today's Review Quota so I can't sidestep the cap by switching modes.
19. As a leetcode-anki user, I want `--review-due 0` and `--review-new 0` to keep working as "skip this bucket entirely" so I can opt out of one bucket without losing the other.
20. As a leetcode-anki user reading `CONTEXT.md`, I want **Daily Quota**, **Review Quota**, **New Quota**, and **Quota Exhausted** to be canonical terms so the codebase, tickets, and conversations stay aligned.
21. As a leetcode-anki user reading the help text for `--review-due` / `--review-new`, I want the description to say "per-day quota size" rather than "per-Session cap" so the flag's behavior matches its documentation.
22. As a leetcode-anki contributor, I want the quota-math step to be a pure function (cached submissions in, two integers out) so its edge cases are testable without a TUI, without a cache file, and without LeetCode.
23. As a leetcode-anki contributor reading `Session()` six months from now, I want `MaxDue` / `MaxNew` documented as per-day quota sizes (not per-Session caps) so I don't accidentally reintroduce the old semantics.
24. As a leetcode-anki contributor, I want a regression test that opens Review Mode, AC's the queue, "restarts," and asserts the queue stays empty so the original bug doesn't come back.
25. As a leetcode-anki contributor, I want a regression test for the esc-out-of-rating-modal path so the cache-invalidate-on-AC behavior doesn't quietly revert.
26. As a leetcode-anki user, I want `Reviews.Due()` (the unwired global "N due" helper) to keep returning the raw global count for now so this PRD's scope stays bounded; cross-list quota-awareness can wait until that surface actually ships.

## Implementation Decisions

### Domain language

- **Daily Quota** is the canonical term for the per-list, per-day budget. **Review Quota** and **New Quota** are its two buckets. **Quota Exhausted** names the empty-but-capped state. All four added to `CONTEXT.md` (already done).
- "Daily Cap" / "MaxDue cap" / per-Session cap are explicitly avoided — flagged in CONTEXT.md as a renamed-meaning ambiguity. The Go field names `SessionConfig.MaxDue` / `MaxNew` keep their identifiers but their semantics shift from "per-Session cap" to "per-day quota size."
- ADR-0001 records the decision and the rejected alternatives (per-session, persisted-session, global, UTC midnight, Review-Mode-only counting).

### Quota math

- A new pure function inside `internal/sr` takes a slug → cached-submissions map (the slugs in the current Problem List), the SR scheduler, and `now`, and returns `(reviewsDoneToday, newDoneToday)` for that list.
- "Review done today" = an Accepted Submit whose `OccurredAt` falls in `[localMidnight, now]` AND whose slug had at least one earlier Accepted Submit AND, at the moment of that AC, the slug's SM-2 schedule (computed over its prior reviews) had `NextDue ≤ at`.
- "New done today" = the slug's first-ever Accepted Submit, with `OccurredAt` in `[localMidnight, now]`.
- Neither category counts: AC's of slugs that were tracked but not yet due (voluntary re-practice), Wrong-Answer Submits, ACs from previous days.
- Day boundary uses `time.Now()` and `OccurredAt` interpreted in `time.Local`. The submission cache stores `OccurredAt` as `time.Unix(ts, 0)` (local zone), so comparisons are direct.
- Function is pure (no I/O, no side-effects). All inputs are passed in.

### Session builder

- `Reviews.Session()` keeps its signature. Internally, after fetching UserProgress and resolving the cache, it calls the quota math to derive `reviewsDoneToday` / `newDoneToday` for `cfg.Slugs`.
- The effective per-bucket budget is `max(0, cfg.MaxDue - reviewsDoneToday)` / `max(0, cfg.MaxNew - newDoneToday)`.
- The walk over `cfg.Slugs` keeps its current "due first, then new" ordering. `DueTotal` / `NewTotal` continue to report the *uncapped* counts (so the footer can say "of 7"); the visible items are now bounded by the effective budget rather than the raw cap.
- `Session` gains three fields:
  - `QuotaExhausted bool` — true when `len(Items) == 0` AND at least one slug in `cfg.Slugs` would have qualified for either bucket but the effective budget for that bucket was zero. Distinct from "no due-or-new work in the list at all."
  - `ReviewsDoneToday int` — the consumed-today counter for the Review Quota.
  - `NewDoneToday int` — the consumed-today counter for the New Quota.
- The "fresh AC since the cache last refreshed" path remains correct because of the cache-invalidate-on-AC change below.

### Cache invalidation lifecycle

- `Reviews.Record()` is split. A new `Reviews.MarkAccepted(slug)` invalidates the cached slug. `Record(slug, submissionID, rating, at)` is reduced to writing the `[anki:N]` note when `rating != 0`; the cache `delete` already happened in `MarkAccepted`.
- TUI `submitResultMsg` Accepted branch calls `MarkAccepted` synchronously *before* it opens the rating modal. Esc'ing the modal therefore still results in a fresh cache for that slug on the next `Status()` / `Session()` call.
- `Record()` is allowed to no-op the rating==0 case the same way it does today (the modal calls it with the chosen rating; non-Accepted Submits don't reach this path).

### TUI

- `loadProblemsCmd` already threads `Session` to the Problems screen. No new wiring at the model level.
- The Problems screen's empty-state and footer logic moves into a small helper (e.g. `quotaFooter(*sr.Session, time.Time)` and a similar empty-state picker). The helper is callable in tests without spinning up the TUI.
- Empty-state copy:
  - `QuotaExhausted == true` → "you've reviewed your daily quota — next batch unlocks at midnight" (or similar).
  - Otherwise → existing/empty pane (current behavior — no work to do in this list right now).
- Footer copy: `<reviewsDone> of <reviewQuota> reviews done today · <newDone> of <newQuota> new`. Visible whenever `m.reviewMode == true` and `m.session != nil`.

### CLI / config

- Flag names `--review-due` / `--review-new` are unchanged (preserving compatibility with anyone who scripted them).
- Help text in `cmd/leetcode-anki/main.go` shifts from "per-Session cap" wording to "per-day quota size."

### Out of scope for this PRD

- **Global (cross-list) quota.** The unwired `Reviews.Due()` stays raw; cross-list "you have N due" badges are deferred until that surface ships.
- **Configurable day boundary.** Local midnight is hardcoded.
- **"Study more" / quota override.** No keystroke or flag to bypass the daily cap on demand.
- **Per-quota copy customization.** Empty-state and footer strings are project-chosen, not user-configurable.
- **Migrating the SR cache schema.** The new cap math reads existing fields; no on-disk format change.

## Testing Decisions

A good test for this work exercises the **observable behavior** ("after AC'ing today's quota, reopening Review Mode keeps the queue empty until local midnight"), not the implementation ("`Session.QuotaExhausted` is set to true"). The boundary the user crosses is what matters; the field is just how we communicate it. Tests should drive against the public surface of each module — pure-function inputs/outputs for the quota math, the `Reviews` interface for `Session`, the rendered model for the TUI — and avoid coupling to internals (cache file shape, intermediate counter struct names, etc.).

### Modules under test

- **Quota math** (pure function in `internal/sr`). Exhaustive table tests covering: empty submissions; one AC today on previously-due → +1 review; one AC today on previously-not-yet-due → 0 (voluntary practice); first-ever AC today → +1 new; AC at local midnight (boundary inclusion); AC just before local midnight → 0; multiple ACs on the same slug same day → counted once; Wrong-Answer Submits today → 0; ACs from yesterday → 0. Deep module: pure inputs in, two integers out, no side effects.
- **`Reviews.Session` integration** with the quota math. Cases: "2 reviews already done today, 5 due in list → 0 items, QuotaExhausted=true, ReviewsDoneToday=2"; "1 done today, 5 due → 1 item, QuotaExhausted=false"; "new quota burned, due quota fresh → no new item, due item still surfaces"; "no candidates at all in list → empty, QuotaExhausted=false (nothing to exhaust)." Uses the existing fake `SubmissionsClient` pattern.
- **Cache invalidation on AC** (TUI-driven). Scenarios: AC + grade → cache invalidated, today's AC visible to next `Session`; AC + esc rating modal → cache still invalidated, today's AC still visible. Asserted via observable Review Mode behavior on a synthetic restart, not by poking at cache internals.
- **TUI empty-state and footer rendering**. Render the Problems screen with a `Session` carrying `QuotaExhausted=true` (and various consumed-today counters) and assert: the empty pane shows the quota-exhausted copy; the footer shows the budget-aware string; both update as items are AC'd within the same session. Render with `QuotaExhausted=false` and the empty pane shows the existing (no-work-here) state.
- **No regression in Explore Mode.** Quota changes are scoped to `reviewMode == true`; Explore Mode rendering and behavior are unchanged.

### Prior art

- `internal/sr/sm2_test.go` is the closest pattern for the pure-function table tests on the quota math: small inputs, named cases, deterministic outputs.
- `internal/sr/reviews_test.go` already drives `Session()` against a fake `SubmissionsClient`; new cases extend that suite.
- `internal/tui/review_test.go` and `internal/tui/review_edit_flow_test.go` are the closest pattern for TUI-driven AC + rating-modal tests; the new esc-out-of-modal test extends that style.
- `internal/tui/problems_view_test.go` is the closest pattern for asserting rendered Problems-screen content; the new empty-state and footer tests extend that style.

## Out of Scope

- Global (cross-list) daily quota. `Reviews.Due()` stays raw; the cross-list "N due" badge it was designed for is not part of this work.
- Configurable day boundary. Local midnight is hardcoded; UTC and rolling-24h variants are explicitly rejected in ADR-0001.
- "Study more" / quota override mechanism. No way to bypass the daily cap on demand.
- New persistence. The quota math is computed from the existing SR submission cache; no new on-disk format, no schema migration.
- Per-Problem-List quota customization. All lists share the same `--review-due` / `--review-new` defaults; per-list overrides are a future feature.
- Quota-aware rendering on the Lists screen. Only the Problems screen (Review Mode) gets new copy and the new footer.
- Notifications / "you have reviews due" reminders. The quota mechanism is passive; the user opens the TUI on their schedule.

## Further Notes

- ADR-0001 (`docs/adr/0001-daily-quota-for-review-mode.md`) is authoritative for the design rationale. This PRD is the implementation-level brief; the ADR is the "why we picked this path over the four alternatives" record.
- The quota-math change and the cache-invalidate-on-AC change are tightly coupled (the latter is required for the former to be correct under the esc-out-of-rating path). They should ship together — splitting them would either ship a known-broken quota or fix a latent bug in isolation without the test coverage that motivates fixing it.
- Default bucket sizes (`defaultReviewDue = 2`, `defaultReviewNew = 1`) are not changed by this PRD. If experience with the daily-quota framing reveals that 2+1 is too low for a daily routine, that's a separate tuning conversation.
- The test account / live contract setup (`internal/leetcode/contracttest`) is not affected: the quota math is a pure function over already-cached data, so no new LeetCode endpoint or mutation is involved.

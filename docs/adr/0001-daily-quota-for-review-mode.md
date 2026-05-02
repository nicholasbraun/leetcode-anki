---
status: accepted
---

# Daily Quota for Review Mode

Review Mode caps how much SR work the user takes on per day. The cap is **per Problem List**, resets at **local midnight**, and is split into two independent buckets: a **Review Quota** (default 2) for previously-AC'd Problems and a **New Quota** (default 1) for never-AC'd ones. A bucket is consumed by today's Accepted Submits — Review Quota by ACs of slugs that were Tracked-and-Due at AC time, New Quota by first-ever ACs — counted from the existing SR submission cache, not from a separate persistence layer. Once a bucket is zero, Review Mode keeps the list's queue empty for that bucket until midnight rolls (Quota Exhausted state); the Problems screen renders a quota-aware empty message and a "X of Y done today" footer so the user can tell "done for the day" from "no work scheduled."

## Considered Options

- **Per-Session caps (the prior behavior)**: `MaxDue` / `MaxNew` applied at every Session-load. Rejected — closing and reopening the TUI on the same day refilled the queue with the next batch, which broke the "I'm done for today" mental model and was the bug that prompted this ADR.
- **Persisted Session ("today's queue, frozen")**: lock the first Session of the day to disk and replay it on subsequent loads even after items are AC'd. Rejected — the user explicitly wanted the cleared queue to stay *empty*, not stay populated with already-done items, and this option would have required new on-disk state for something derivable from existing submission timestamps.
- **Global (cross-list) quota**: one budget across all Problem Lists. Rejected — `Session()` is per-list today, the user runs Review Mode one list at a time, and Anki's per-deck convention is the closest existing mental model. Per-list also avoids cross-list bookkeeping in the SR builder.
- **UTC midnight or rolling 24h boundary**: rejected for the obvious reason that the quota's job is to map to "a day" as the user lives it. The submission cache stores `OccurredAt` in local zone, so local-midnight comparisons are also the cheapest implementation.
- **Count only Review-Mode ACs (not Explore-Mode ACs)**: rejected because (a) the cache has no "which mode produced this AC" flag and adding one means new state; (b) the quota models cognitive load, which doesn't depend on which screen the user was on; (c) it would let a user side-step the quota by always submitting via Explore Mode.

## Consequences

- `SessionConfig.MaxDue` / `MaxNew` keep their Go names but their **meaning** shifts from "per-Session cap" to "per-day bucket size." Any future caller that conflates these with per-load limits will produce wrong quotas. CONTEXT.md flags this as a renamed-meaning ambiguity.
- The SR cache must reflect today's ACs for the quota math to be correct. The pre-existing behavior — `Reviews.Record()` invalidates the slug only when the user grades, so esc'ing the rating modal leaves a stale cache — is a latent bug that becomes user-visible under this design (an esc'd review wouldn't burn quota). Cache invalidation is therefore moved to fire on every Accepted Submit, ahead of the rating modal; `Record()` is reduced to writing the `[anki:N]` note.
- `Session` gains a `QuotaExhausted` (or equivalent) signal so the view layer can pick the empty-state copy without re-deriving the quota math. `DueTotal` / `NewTotal` are joined by per-bucket "consumed today" counts for the footer.
- `Reviews.Due()` (the unwired global "N due" helper) stays raw — quota-awareness for cross-list affordances is deferred until that surface actually ships.
- A user crossing timezones can get a slightly larger or smaller quota window for one day. Accepted as a personal-tool trade-off; revisit only if it shows up as a real complaint.

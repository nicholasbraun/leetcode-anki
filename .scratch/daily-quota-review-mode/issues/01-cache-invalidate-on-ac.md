# Cache-invalidate on every Accepted Submit

Status: needs-triage

## Parent

[../PRD.md](../PRD.md)

## What to build

The SR submission cache currently only invalidates a slug when `Reviews.Record()` is called, and `Record()` only fires when the user picks a grade in the rating modal. If the user AC's a Submit and then `esc`'s out of the modal without rating, the cache for that slug stays frozen at its pre-AC state — indefinitely, until the user grades something on it later. This is a pre-existing latent bug; the daily-quota work in #02 will surface it as a user-visible quota-counting error, but it's worth fixing on its own.

This slice splits the current Record-on-grade affordance into two: a new `Reviews.MarkAccepted(slug)` that invalidates the slug's cache unconditionally on every AC, and a slimmed-down `Record(slug, submissionID, rating, at)` that only writes the `[anki:N]` note. The TUI calls `MarkAccepted` synchronously from the `submitResultMsg` Accepted branch, **before** the rating modal opens. From there on, esc'd reviews still result in a fresh cache on the next `Status()` / `Session()` call.

### `Reviews` interface

- New method:
  ```
  // MarkAccepted invalidates the cached submission timeline for slug so
  // the next Status / Session call refetches. Called on every Accepted
  // Submit, regardless of whether the user grades the rating modal.
  MarkAccepted(ctx context.Context, slug string) error
  ```
- `Record` shrinks to: write the `[anki:N]` note when `rating != 0`, return nil otherwise. The `delete(r.cache.Slugs, slug)` and `r.cache.save(r.path)` calls move into `MarkAccepted` (not `Record`). `Record` no longer needs to invalidate — `MarkAccepted` already did.
- Compile-time check (`var _ Reviews = (*reviews)(nil)`) keeps both methods on the production type.

### TUI wiring

- In `internal/tui/app.go`, the `submitResultMsg` Accepted branch calls `m.reviews.MarkAccepted(m.ctx, slug)` synchronously, ahead of opening the rating modal.
- Errors from `MarkAccepted` are surfaced as `m.err` (consistent with how other SR errors are surfaced today). Failing to invalidate must not block the modal from opening — the user's AC stands either way.
- `Record()`'s call site in `commitGrade` (result_view.go) is unchanged structurally; it just no longer triggers cache invalidation as a side-effect.

### Domain documentation

- No `CONTEXT.md` changes — this is an implementation refactor, not a domain-language change. The existing `Review` term still describes the behavior end-to-end.

## Acceptance criteria

### Reviews interface

- [ ] `MarkAccepted(slug)` removes the slug from the in-memory cache and persists the cache to disk.
- [ ] `MarkAccepted(slug)` on a slug not in the cache is a no-op (no error).
- [ ] `Record(slug, submissionID, rating, at)` with `rating != 0` still writes the `[anki:N]` note via `UpdateSubmissionNote`.
- [ ] `Record(slug, submissionID, 0, at)` is a no-op (no note write, no cache change).
- [ ] After `MarkAccepted(slug)` then `Status(ctx, slug, now)`, the cache is repopulated from `SubmissionList` (cache miss path exercised).

### TUI behavior

- [ ] On `submitResultMsg` Accepted, `MarkAccepted(slug)` is invoked exactly once before the rating modal renders.
- [ ] Esc-out of the rating modal: the cache for that slug is invalidated. Next `Session()` build reflects today's AC.
- [ ] Commit a grade in the rating modal: `Record` is invoked with the chosen rating; the note is written; cache is already empty (from `MarkAccepted`); next `Session()` reflects today's AC.
- [ ] `MarkAccepted` returning an error sets `m.err` but does not block the rating modal from opening.

### Quality gates

- [ ] `go vet ./... && go build ./... && go test ./...` all pass before commit.
- [ ] All commits in this slice are individually atomic — `vet`, `build`, and `test` pass at every commit, not only at the end.
- [ ] New tests follow the TDD red-green-refactor loop per the project convention.

## Blocked by

None — can start immediately.

// Package sr implements spaced-repetition Review Mode for leetcode-anki.
//
// SR state lives entirely on LeetCode (the user's submission history is
// the source of truth); this package adds a scheduler on top of that
// timeline and an [anki:N] tag in submission notes for explicit grading.
// The on-disk cache memoizes per-Problem submission timelines so Review
// Mode entry doesn't refetch the world every session.
package sr

import (
	"context"
	"sort"
	"time"

	"leetcode-anki/internal/leetcode"
)

// Reviews is the TUI-facing surface. Mirrors the LeetcodeClient /
// SolutionCache / Editor injection pattern in internal/tui/client.go.
type Reviews interface {
	// Record attaches an explicit grade to a just-completed Accepted
	// submission. rating == 0 means "no explicit grade" — Status will
	// fall back to the implicit Accepted-as-Good mapping. Either way,
	// the slug's cache is invalidated so next Status refreshes.
	Record(ctx context.Context, slug, submissionID string, rating int, at time.Time) error

	// Status returns the SR state for a Problem at time `now`.
	// Used to render the "next review in X" badge in lists and to filter
	// the problems screen in Review Mode (see Status.Due).
	Status(ctx context.Context, slug string, now time.Time) (Status, error)

	// Due returns every Problem currently due for review at `now`,
	// sourced from the global UserProgress candidate set. This is the
	// Review Mode entry point — the TUI calls it when the user presses
	// the Review key on the lists screen.
	Due(ctx context.Context, now time.Time) ([]DueProblem, error)

	// Preview forecasts the next-due time for each candidate rating
	// (index 0..3 = ratings 1..4 / Again/Hard/Good/Easy) if the user
	// were to grade `slug` at `now`. Powers the rating modal's "due in
	// X days" hint. Returns the zero array on error.
	Preview(ctx context.Context, slug string, now time.Time) ([4]time.Time, error)
}

// DueProblem is one entry in the Review Mode list. Carries enough display
// metadata for the TUI to render rows without re-fetching question detail.
type DueProblem struct {
	TitleSlug  string
	Title      string
	FrontendID string
	Difficulty string
	NextDue    time.Time
	Reviews    int
}

// Status is the SR snapshot for a single Problem.
type Status struct {
	Tracked bool      // false => no Accepted submission yet
	NextDue time.Time // zero when !Tracked
	Reviews int       // count of reviews folded into the schedule
}

// Due reports whether a Problem is due for review at `now`. False for
// Problems that aren't yet tracked (no Accepted submission).
func (s Status) Due(now time.Time) bool {
	return s.Tracked && !now.Before(s.NextDue)
}

// SubmissionsClient is the slice of *leetcode.Client this package needs.
// Production wiring satisfies it via the concrete client; tests inject a
// fake that returns canned data.
type SubmissionsClient interface {
	SubmissionList(ctx context.Context, slug, nextKey string, limit int) ([]leetcode.Submission, string, error)
	UserProgress(ctx context.Context, skip, limit int) ([]leetcode.ProgressQuestion, int, error)
	UpdateSubmissionNote(ctx context.Context, submissionID, note string, tagIDs []int, flagType string) error
}

// Compile-time check that the production client satisfies the interface,
// matching the pattern in internal/tui/client.go:42-44.
var _ SubmissionsClient = (*leetcode.Client)(nil)

// Open loads (or creates) the cache and returns a Reviews ready for use.
// Scheduler choice is private (SM-2 today); callers can't see it.
func Open(lc SubmissionsClient) (Reviews, error) {
	path, err := defaultCachePath()
	if err != nil {
		return nil, err
	}
	cache, err := loadCache(path)
	if err != nil {
		return nil, err
	}
	return &reviews{
		lc:    lc,
		sched: sm2{},
		cache: cache,
		path:  path,
	}, nil
}

type reviews struct {
	lc    SubmissionsClient
	sched scheduler
	cache *cacheData
	path  string
}

// Record handles the verdict-detection-site call. v1 assumes the
// submission is brand-new (no prior user-written notes, default
// flagType). If a future "regrade past reviews" feature lands, it must
// refresh the cache and round-trip Notes/FlagType from the latest
// SubmissionList read before calling UpdateSubmissionNote — otherwise
// it'll clobber user customizations.
func (r *reviews) Record(ctx context.Context, slug, submissionID string, rating int, at time.Time) error {
	if rating != 0 {
		note := applyTag("", rating)
		if err := r.lc.UpdateSubmissionNote(ctx, submissionID, note, []int{}, "WHITE"); err != nil {
			return err
		}
	}
	delete(r.cache.Slugs, slug)
	return r.cache.save(r.path)
}

func (r *reviews) Status(ctx context.Context, slug string, now time.Time) (Status, error) {
	entry, ok := r.cache.Slugs[slug]
	if !ok {
		if err := r.refreshSlug(ctx, slug); err != nil {
			return Status{}, err
		}
		entry = r.cache.Slugs[slug]
	}

	revs := buildReviews(entry.Submissions)
	if len(revs) == 0 {
		return Status{Tracked: false}, nil
	}
	return Status{
		Tracked: true,
		NextDue: r.sched.schedule(revs),
		Reviews: len(revs),
	}, nil
}

// Due pages through UserProgress, filters to AC Problems, computes Status
// for each (cache-hit on warm slugs, fetch on cold), and keeps those whose
// NextDue is at or before `now`. May make many API calls on a cold cache;
// subsequent calls hit cache.
func (r *reviews) Due(ctx context.Context, now time.Time) ([]DueProblem, error) {
	const limit = 50
	var allProgress []leetcode.ProgressQuestion
	skip := 0
	for {
		page, total, err := r.lc.UserProgress(ctx, skip, limit)
		if err != nil {
			return nil, err
		}
		allProgress = append(allProgress, page...)
		skip += len(page)
		if len(page) == 0 || skip >= total {
			break
		}
	}

	out := make([]DueProblem, 0, len(allProgress))
	for _, p := range allProgress {
		if !p.LastAccepted {
			continue
		}
		st, err := r.Status(ctx, p.TitleSlug, now)
		if err != nil {
			continue // skip the slug; don't fail the whole Due query
		}
		if !st.Due(now) {
			continue
		}
		out = append(out, DueProblem{
			TitleSlug:  p.TitleSlug,
			Title:      p.Title,
			FrontendID: p.FrontendID,
			Difficulty: p.Difficulty,
			NextDue:    st.NextDue,
			Reviews:    st.Reviews,
		})
	}
	return out, nil
}

// Preview runs the scheduler against the cached history plus a synthetic
// review at `now` for each candidate rating. The cache is warmed via
// Status, so the first call for a slug pays the SubmissionList round-trip
// just like Status does; subsequent calls are pure CPU.
func (r *reviews) Preview(ctx context.Context, slug string, now time.Time) ([4]time.Time, error) {
	var out [4]time.Time
	if _, err := r.Status(ctx, slug, now); err != nil {
		return out, err
	}
	base := buildReviews(r.cache.Slugs[slug].Submissions)
	for i, rating := range [4]int{1, 2, 3, 4} {
		synthetic := append(append([]review{}, base...),
			review{at: now, quality: ratingToQuality(rating, true)})
		out[i] = r.sched.schedule(synthetic)
	}
	return out, nil
}

// refreshSlug pulls the full submission timeline for a Problem and
// replaces the cached entry. Pages through SubmissionList until lastKey
// is empty; persists the cache before returning.
func (r *reviews) refreshSlug(ctx context.Context, slug string) error {
	var all []leetcode.Submission
	nextKey := ""
	for {
		page, nk, err := r.lc.SubmissionList(ctx, slug, nextKey, 50)
		if err != nil {
			return err
		}
		all = append(all, page...)
		if nk == "" {
			break
		}
		nextKey = nk
	}

	cached := make([]cachedSubmission, 0, len(all))
	for _, s := range all {
		cached = append(cached, cachedSubmission{
			ID:         s.ID,
			OccurredAt: s.OccurredAt,
			Accepted:   s.Accepted,
			Notes:      s.Notes,
			FlagType:   s.FlagType,
		})
	}
	r.cache.Slugs[slug] = slugEntry{
		FetchedAt:   time.Now(),
		Submissions: cached,
	}
	return r.cache.save(r.path)
}

// buildReviews converts a Problem's cached submission timeline into the
// scheduler's input. Submissions before the first Accepted one are
// pre-baseline and excluded — a Problem the user struggled with for
// weeks before solving shouldn't enter SR with dozens of phantom failures.
// From the first Accepted onwards, everything counts: subsequent failures
// reset the SM-2 interval.
func buildReviews(submissions []cachedSubmission) []review {
	sorted := append([]cachedSubmission{}, submissions...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].OccurredAt.Before(sorted[j].OccurredAt) })

	out := make([]review, 0, len(sorted))
	seenAC := false
	for _, s := range sorted {
		if !seenAC {
			if !s.Accepted {
				continue
			}
			seenAC = true
		}
		rating, _ := parseTag(s.Notes)
		out = append(out, review{at: s.OccurredAt, quality: ratingToQuality(rating, s.Accepted)})
	}
	return out
}

// ratingToQuality maps the user's [anki:N] grade (or implicit-from-Verdict
// fallback) onto SM-2's 0-5 quality scale. Standard Anki-style mapping:
// 1=Again, 2=Hard, 3=Good (default), 4=Easy. Failures within review history
// are q=1 regardless of rating.
func ratingToQuality(rating int, accepted bool) int {
	if !accepted {
		return 1
	}
	switch rating {
	case 1:
		return 1
	case 2:
		return 3
	case 3:
		return 4
	case 4:
		return 5
	default:
		return 4 // implicit
	}
}

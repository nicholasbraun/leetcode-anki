package sr

import "time"

// review is the scheduler's input unit: when the user reviewed a Problem
// and how well they did, on SuperMemo's 0-5 quality scale. The conversion
// from leetcode.Submission (Accepted + optional [anki:N] tag) lives in
// the Reviews facade, not here — the scheduler stays purely numerical.
type review struct {
	at      time.Time
	quality int
}

// scheduler decides when a Problem is next due, given its review history.
// Implementations must be pure: no I/O, deterministic, idempotent over
// the same input. SR's algorithm-deferral story rests on swapping this
// type behind an unchanged Reviews facade.
type scheduler interface {
	schedule(reviews []review) time.Time
}

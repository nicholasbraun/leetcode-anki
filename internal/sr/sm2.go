package sr

import "time"

// sm2 implements the SuperMemo-2 spacing algorithm. Pure function over
// review history: walks reviews chronologically, compounds ease factor
// and interval, and returns next-due as last-review-at + interval days.
type sm2 struct{}

const (
	sm2InitialEF = 2.5
	sm2MinEF     = 1.3
)

func (sm2) schedule(reviews []review) time.Time {
	if len(reviews) == 0 {
		return time.Time{}
	}

	ef := sm2InitialEF
	interval := 0
	n := 0

	for _, r := range reviews {
		q := r.quality
		if q < 3 {
			n = 0
			interval = 1
		} else {
			switch n {
			case 0:
				interval = 1
			case 1:
				interval = 6
			default:
				interval = int(float64(interval)*ef + 0.5)
			}
			n++
		}

		ef = ef + (0.1 - float64(5-q)*(0.08+float64(5-q)*0.02))
		if ef < sm2MinEF {
			ef = sm2MinEF
		}
	}

	return reviews[len(reviews)-1].at.AddDate(0, 0, interval)
}

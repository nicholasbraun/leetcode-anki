package sr

import "time"

// sm2 implements an Anki-flavored SuperMemo-2 spacing algorithm: plain SM-2
// for ease-factor evolution, plus per-rating interval multipliers so that
// Hard, Good, and Easy actually produce different next-due dates on the
// same review (vanilla SM-2 only diverges over compounding ease updates,
// which makes a per-rating preview meaningless).
type sm2 struct{}

const (
	sm2InitialEF = 2.5
	sm2MinEF     = 1.3
	// sm2HardMult shrinks the next interval after a Hard rating to 1.2× the
	// previous one (with a +1d floor, mirroring Anki's minimum-progress rule).
	sm2HardMult = 1.2
	// sm2EasyBonus stretches the next interval after an Easy rating beyond
	// the plain ef-scaled value, matching Anki's default 1.3 bonus.
	sm2EasyBonus = 1.3
	// sm2EasyGraduate is the first interval awarded to a freshly-baselined
	// Problem if the very first Accepted submission was rated Easy. Hard and
	// Good still graduate at 1 day (the SM-2 baseline); Easy jumps ahead.
	sm2EasyGraduate = 4
	// sm2HardSecond is the interval awarded on the second review when the
	// user picks Hard — keeps Hard short without collapsing back to 1d.
	sm2HardSecond = 3
	// sm2EasySecond mirrors sm2HardSecond on the Easy side; standard SM-2's
	// second interval is 6d, Easy stretches that to 8d.
	sm2EasySecond = 8
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
		switch {
		case q < 3:
			n = 0
			interval = 1
		case n == 0:
			if q == 5 {
				interval = sm2EasyGraduate
			} else {
				interval = 1
			}
			n++
		case n == 1:
			switch q {
			case 3:
				interval = sm2HardSecond
			case 5:
				interval = sm2EasySecond
			default:
				interval = 6
			}
			n++
		default:
			switch q {
			case 3:
				next := int(float64(interval)*sm2HardMult + 0.5)
				if next <= interval {
					next = interval + 1
				}
				interval = next
			case 5:
				interval = int(float64(interval)*ef*sm2EasyBonus + 0.5)
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

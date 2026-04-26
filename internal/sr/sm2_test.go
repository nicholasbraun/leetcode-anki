package sr

import (
	"testing"
	"time"
)

// SM-2 documented behavior:
//   1st pass:           interval = 1 day
//   2nd pass:           interval = 6 days
//   3rd+ pass:          interval = previous * EF
//   any q<3 (failure):  interval = 1 day, n reset
//   EF floor:           1.3

func TestSM2_FirstReviewIsOneDay(t *testing.T) {
	s := sm2{}
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	got := s.schedule([]review{{at: at, quality: 4}})
	want := at.AddDate(0, 0, 1)
	if !got.Equal(want) {
		t.Errorf("nextDue = %v, want %v", got, want)
	}
}

func TestSM2_SecondPassIsSixDays(t *testing.T) {
	s := sm2{}
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 0, 1)
	got := s.schedule([]review{
		{at: first, quality: 4},
		{at: second, quality: 4},
	})
	want := second.AddDate(0, 0, 6)
	if !got.Equal(want) {
		t.Errorf("nextDue = %v, want %v", got, want)
	}
}

// After a third "Good" pass, interval becomes 6 * EF days. Initial EF is
// 2.5; q=4 leaves it unchanged (the EF delta at q=4 is 0.0). So third pass
// interval should land at 15 days.
func TestSM2_ThirdPassUsesEaseFactor(t *testing.T) {
	s := sm2{}
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 0, 1)
	third := second.AddDate(0, 0, 6)
	got := s.schedule([]review{
		{at: first, quality: 4},
		{at: second, quality: 4},
		{at: third, quality: 4},
	})
	want := third.AddDate(0, 0, 15)
	if !got.Equal(want) {
		t.Errorf("nextDue = %v, want %v", got, want)
	}
}

// A failure mid-sequence must reset the interval to 1 day. Without this,
// a Wrong-Answer submission would still schedule weeks out, defeating SR.
func TestSM2_FailureResetsToOneDay(t *testing.T) {
	s := sm2{}
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second := first.AddDate(0, 0, 1)
	third := second.AddDate(0, 0, 6)
	fourth := third.AddDate(0, 0, 14)
	got := s.schedule([]review{
		{at: first, quality: 4},
		{at: second, quality: 4},
		{at: third, quality: 4},
		{at: fourth, quality: 1},
	})
	want := fourth.AddDate(0, 0, 1)
	if !got.Equal(want) {
		t.Errorf("nextDue = %v, want %v", got, want)
	}
}

// EF must not drift below 1.3. Hammering with q=0 reviews would otherwise
// produce nonsensical (negative) ease factors and intervals.
func TestSM2_EaseFactorFloor(t *testing.T) {
	s := sm2{}
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reviews := make([]review, 0, 20)
	for i := range 20 {
		reviews = append(reviews, review{at: at.AddDate(0, 0, i), quality: 0})
	}
	got := s.schedule(reviews)
	// After 20 failures, interval is reset (q<3) every time, so nextDue
	// is always lastAt + 1. The point of the test is that schedule didn't
	// panic or produce a wildly past time from a negative-EF interval.
	want := reviews[len(reviews)-1].at.AddDate(0, 0, 1)
	if !got.Equal(want) {
		t.Errorf("nextDue after 20 failures = %v, want %v", got, want)
	}
}

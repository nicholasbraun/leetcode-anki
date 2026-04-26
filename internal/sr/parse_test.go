package sr

import "testing"

func TestParseTag(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantRating int
		wantOK     bool
	}{
		{"empty", "", 0, false},
		{"plain notes only", "TEST", 0, false},
		{"only tag", "[anki:3]", 3, true},
		{"prefix and tag", "TEST\n[anki:2]", 2, true},
		{"out of range high", "[anki:5]", 0, false},
		{"out of range zero", "[anki:0]", 0, false},
		{"junk format", "[anki:abc]", 0, false},
		{"tag embedded mid-string", "before [anki:1] after", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := parseTag(tt.in)
			if r != tt.wantRating || ok != tt.wantOK {
				t.Errorf("parseTag(%q) = (%d, %v), want (%d, %v)", tt.in, r, ok, tt.wantRating, tt.wantOK)
			}
		})
	}
}

func TestStripTag(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"TEST", "TEST"},
		{"[anki:3]", ""},
		{"TEST\n[anki:3]", "TEST"},
		{"TEST [anki:3]", "TEST"},
		// Lenient: strip all anki tags rather than only one. Defends against
		// a stale double-write that left two tags in the note.
		{"[anki:1]\n[anki:2]", ""},
	}
	for _, tt := range tests {
		if got := stripTag(tt.in); got != tt.want {
			t.Errorf("stripTag(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// applyTag must preserve the user's free-text prefix and replace any prior
// [anki:N] rather than stacking tags on top of each other on every Record.
func TestApplyTag(t *testing.T) {
	tests := []struct {
		notes  string
		rating int
		want   string
	}{
		{"", 3, "[anki:3]"},
		{"TEST", 3, "TEST\n[anki:3]"},
		{"TEST\n[anki:1]", 3, "TEST\n[anki:3]"},
		{"[anki:2]", 4, "[anki:4]"},
	}
	for _, tt := range tests {
		if got := applyTag(tt.notes, tt.rating); got != tt.want {
			t.Errorf("applyTag(%q, %d) = %q, want %q", tt.notes, tt.rating, got, tt.want)
		}
	}
}

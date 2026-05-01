package modal

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRender(t *testing.T) {
	cases := []struct {
		name        string
		opts        Options
		wantContain []string
	}{
		{
			name: "lang-picker shape: tight padding, short body",
			opts: Options{
				Body:   "pick a language\n\n  golang\n  python",
				Width:  80,
				Height: 24,
				PadV:   0,
				PadH:   2,
				Footer: " enter  select    esc  cancel",
			},
			wantContain: []string{
				"pick a language",
				"golang",
				"python",
				"enter",
				"esc",
			},
		},
		{
			name: "grade-modal shape: roomy padding, multi-line body",
			opts: Options{
				Body:   "ACCEPTED\n\nhow confidently did you solve it?\n\n  1  Again\n  2  Hard\n  3  Good\n  4  Easy",
				Width:  120,
				Height: 30,
				PadV:   1,
				PadH:   3,
				Footer: " 1-4  rate    enter  pick    esc  cancel",
			},
			wantContain: []string{
				"ACCEPTED",
				"how confidently",
				"Again", "Hard", "Good", "Easy",
				"1-4",
				"rate",
			},
		},
		{
			name: "narrow terminal still renders body and footer",
			opts: Options{
				Body:   "hi\nthere",
				Width:  40,
				Height: 12,
				PadV:   0,
				PadH:   1,
				Footer: " esc  cancel",
			},
			wantContain: []string{"hi", "there", "esc", "cancel"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(tc.opts)
			for _, want := range tc.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\nfull output:\n%s", want, got)
				}
			}

			// Footer must be the last line; everything above it is the
			// centered modal placement.
			lines := strings.Split(got, "\n")
			if len(lines) < 2 {
				t.Fatalf("expected at least 2 lines (placed + footer), got %d:\n%s", len(lines), got)
			}
			lastLine := lines[len(lines)-1]
			if !strings.Contains(lastLine, strings.TrimSpace(tc.opts.Footer)) {
				t.Errorf("footer should be on the last line; got last=%q want substring %q", lastLine, tc.opts.Footer)
			}

			// Centering sanity: lipgloss.Place fills (width × height-1) so
			// the rendered placed-block has exactly height-1 lines, then
			// one line of footer = height total.
			if len(lines) != tc.opts.Height {
				t.Errorf("line count = %d, want %d (height-1 placed + 1 footer)", len(lines), tc.opts.Height)
			}

			// Width sanity: every line should fit within the requested
			// width. lipgloss.Place pads to exactly width columns.
			for i, ln := range lines[:len(lines)-1] {
				if w := lipgloss.Width(ln); w != tc.opts.Width {
					t.Errorf("placed line %d has visible width %d, want %d", i, w, tc.opts.Width)
				}
			}
		})
	}
}

// Padding option must round-trip through to the rendered border so a
// PadH=3 modal is visibly wider than a PadH=0 modal with the same body.
func TestRender_PaddingAffectsWidth(t *testing.T) {
	body := "x"
	tight := Render(Options{Body: body, Width: 80, Height: 20, PadV: 0, PadH: 0, Footer: ""})
	roomy := Render(Options{Body: body, Width: 80, Height: 20, PadV: 0, PadH: 5, Footer: ""})

	// The placed regions differ in how much non-space content they have
	// (the roomy variant has more padding inside the border, so the
	// border itself is wider). Crude check: count total non-space runes.
	tightContent := strings.Count(tight, "─")
	roomyContent := strings.Count(roomy, "─")
	if roomyContent <= tightContent {
		t.Errorf("expected roomy padding to widen the border (more dashes); tight=%d roomy=%d", tightContent, roomyContent)
	}
}

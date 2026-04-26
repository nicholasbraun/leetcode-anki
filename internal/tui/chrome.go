package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// breadcrumb renders "leetcode-anki › lists › Blind 75" with the trailing
// crumb highlighted. Crumbs degrade for narrow widths: <60 drops the leading
// "leetcode-anki", <40 keeps only the active crumb.
func breadcrumb(width int, crumbs ...string) string {
	if len(crumbs) == 0 || width <= 0 {
		return ""
	}
	if width < 40 {
		crumbs = crumbs[len(crumbs)-1:]
	} else if width < 60 && len(crumbs) > 1 {
		crumbs = crumbs[1:]
	}
	parts := make([]string, 0, len(crumbs)*2)
	for i, c := range crumbs {
		if i > 0 {
			parts = append(parts, breadcrumbSepStyle.Render(" › "))
		}
		if i == len(crumbs)-1 {
			parts = append(parts, breadcrumbActiveStyle.Render(c))
		} else {
			parts = append(parts, breadcrumbStyle.Render(c))
		}
	}
	return " " + strings.Join(parts, "")
}

// divider renders a "╱  label ──────────…" line padded to width. label may
// be empty. splitAt > 0 inserts splitGlyph at that visible column index
// (typically ╮ for top dividers and ┴ for bottom). Out-of-range splitAt is
// silently ignored.
func divider(width int, label string, splitAt int, splitGlyph string) string {
	if width <= 0 {
		return ""
	}
	prefix := dividerAccentStyle.Render(" ╱ ")
	consumed := 3
	body := strings.Builder{}
	body.WriteString(prefix)
	if label != "" {
		labelStr := " " + label + " "
		body.WriteString(dividerAccentStyle.Render(labelStr))
		consumed += lipgloss.Width(labelStr)
	}
	dashCount := width - consumed
	if dashCount < 0 {
		dashCount = 0
	}
	dashes := strings.Repeat("─", dashCount)
	if splitAt > 0 && splitGlyph != "" {
		// splitAt is a column relative to the start of the line. The dash
		// run begins at column `consumed`, so the index into the dash run
		// is splitAt - consumed.
		idx := splitAt - consumed
		if idx >= 0 && idx < dashCount {
			r := []rune(dashes)
			gl := []rune(splitGlyph)
			if len(gl) > 0 {
				r[idx] = gl[0]
			}
			dashes = string(r)
		}
	}
	body.WriteString(dividerLineStyle.Render(dashes))
	return body.String()
}

// footerItem is one (key, label) pair shown in the bottom keybind hint row.
type footerItem struct{ Key, Label string }

// footer renders pinned keybind hints with a 2-space gutter between key and
// label and a 4-space gutter between groups. Truncates with " …" when the
// row would exceed width; never wraps.
func footer(width int, items ...footerItem) string {
	if width <= 0 || len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(" ")
	consumed := 1
	for i, it := range items {
		group := footerKeyStyle.Render(it.Key) + "  " + footerLabelStyle.Render(it.Label)
		gw := lipgloss.Width(group)
		gap := 0
		if i > 0 {
			gap = 4
		}
		if consumed+gap+gw > width-2 {
			b.WriteString(footerLabelStyle.Render(" …"))
			break
		}
		if gap > 0 {
			b.WriteString(strings.Repeat(" ", gap))
			consumed += gap
		}
		b.WriteString(group)
		consumed += gw
	}
	return b.String()
}

// difficultyGlyph maps "EASY"/"MEDIUM"/"HARD" (any case) to the colored
// ◔ ◑ ● glyph. Unknown values render as a dimmed bullet.
func difficultyGlyph(d string) string {
	switch strings.ToUpper(d) {
	case "EASY":
		return easyStyle.Render(glyphEasy)
	case "MEDIUM":
		return mediumStyle.Render(glyphMedium)
	case "HARD":
		return hardStyle.Render(glyphHard)
	}
	return dimStyle.Render("·")
}

// difficultyLabel renders the lowercase difficulty word in its color. Unknown
// values render dim.
func difficultyLabel(d string) string {
	switch strings.ToUpper(d) {
	case "EASY":
		return easyStyle.Render("easy")
	case "MEDIUM":
		return mediumStyle.Render("medium")
	case "HARD":
		return hardStyle.Render("hard")
	}
	return dimStyle.Render(strings.ToLower(d))
}

package render

import (
	"strings"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown"
)

// HTMLToMarkdown converts a LeetCode problem's HTML `content` into clean
// markdown suitable for rendering with glamour.
func HTMLToMarkdown(html string) (string, error) {
	conv := htmltomd.NewConverter("", true, nil)
	md, err := conv.ConvertString(html)
	if err != nil {
		return "", err
	}
	// LeetCode descriptions often use <pre> blocks that come out indented;
	// trim trailing whitespace per line to keep glamour rendering tidy.
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n"), nil
}

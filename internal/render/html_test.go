package render

import (
	"strings"
	"testing"
)

func TestHTMLToMarkdownTrimsTrailingWhitespace(t *testing.T) {
	in := "<p>hello   </p><pre>code  \n</pre>"
	got, err := HTMLToMarkdown(in)
	if err != nil {
		t.Fatalf("HTMLToMarkdown: %v", err)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			t.Errorf("line has trailing whitespace: %q", line)
		}
	}
}

func TestMarkdownToTerminalRenders(t *testing.T) {
	out, err := MarkdownToTerminal("# hello\n\nworld\n", 40)
	if err != nil {
		t.Fatalf("MarkdownToTerminal: %v", err)
	}
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Errorf("missing rendered content: %q", out)
	}
}

func TestHTMLToTerminalFallsBackOnBadMarkdown(t *testing.T) {
	out, err := HTMLToTerminal("<p>plain text</p>", 40)
	if err != nil {
		t.Fatalf("HTMLToTerminal: %v", err)
	}
	if !strings.Contains(out, "plain text") {
		t.Errorf("missing content: %q", out)
	}
}

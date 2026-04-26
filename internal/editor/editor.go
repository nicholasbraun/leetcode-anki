package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
)

// LeetCode title slugs are lowercase alphanumerics and hyphens. Anything else
// (path separators, dots, control chars) could escape the cache directory if
// joined into a path, so we reject it at the boundary.
var validSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// extByLangSlug maps LeetCode's `langSlug` values to a sensible file extension.
// Unknown slugs fall back to .txt.
var extByLangSlug = map[string]string{
	"golang":      "go",
	"python":      "py",
	"python3":     "py",
	"cpp":         "cpp",
	"c":           "c",
	"java":        "java",
	"javascript":  "js",
	"typescript":  "ts",
	"rust":        "rs",
	"ruby":        "rb",
	"swift":       "swift",
	"kotlin":      "kt",
	"scala":       "scala",
	"csharp":      "cs",
	"php":         "php",
	"bash":        "sh",
	"mysql":       "sql",
	"mssql":       "sql",
	"oraclesql":   "sql",
	"postgresql":  "sql",
	"elixir":      "ex",
	"erlang":      "erl",
	"racket":      "rkt",
	"dart":        "dart",
}

// Ext returns the file extension (without leading dot) for a given langSlug.
func Ext(langSlug string) string {
	if e, ok := extByLangSlug[langSlug]; ok {
		return e
	}
	return "txt"
}

// SolutionPath returns the on-disk path for a problem's solution file.
// Format: <UserCacheDir>/leetcode-anki/<titleSlug>/solution.<ext>
//
// titleSlug is rejected if it doesn't match LeetCode's slug format, since this
// value is interpolated into a filesystem path.
func SolutionPath(titleSlug, langSlug string) (string, error) {
	if !validSlug.MatchString(titleSlug) {
		return "", fmt.Errorf("invalid title slug %q", titleSlug)
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "leetcode-anki", titleSlug, "solution."+Ext(langSlug)), nil
}

// ExistingSolutionPath returns the on-disk path of a previously-written
// solution for this problem+language, or "" if no file exists. Used to detect
// resumable work without scaffolding a fresh file.
func ExistingSolutionPath(titleSlug, langSlug string) string {
	if langSlug == "" {
		return ""
	}
	p, err := SolutionPath(titleSlug, langSlug)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// ScaffoldFile creates the solution file (with the language's starter snippet)
// if it does not already exist. If the file exists, it is left untouched so the
// user can resume work in progress. Returns the resolved file path.
func ScaffoldFile(titleSlug, langSlug, snippet string) (string, error) {
	path, err := SolutionPath(titleSlug, langSlug)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(snippet), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// resolveEditor picks the user's editor: $VISUAL → $EDITOR → "vi".
func resolveEditor() string {
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v
	}
	return "vi"
}

// EditorDoneMsg is delivered to the Bubble Tea Update loop after the editor exits.
type EditorDoneMsg struct {
	// Path is the file the editor was launched on. Stored for callers that
	// want to read the post-edit contents without re-resolving the path.
	Path string
	// Err is non-nil if the editor exited non-zero or the OS couldn't launch it.
	Err error
}

// OpenInEditor returns a tea.Cmd that suspends the TUI, runs the user's editor
// on `path`, and on exit emits an EditorDoneMsg.
func OpenInEditor(path string) tea.Cmd {
	editor := resolveEditor()
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditorDoneMsg{Path: path, Err: err}
	})
}

// ReadSolution returns the current contents of the solution file.
func ReadSolution(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read solution: %w", err)
	}
	return string(b), nil
}

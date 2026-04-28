package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// LeetCode title slugs are lowercase alphanumerics and hyphens. Anything else
// (path separators, dots, control chars) could escape the cache directory if
// joined into a path, so we reject it at the boundary.
var validSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// extByLangSlug maps LeetCode's `langSlug` values to a sensible file extension.
// Unknown slugs fall back to .txt.
var extByLangSlug = map[string]string{
	"golang":     "go",
	"python":     "py",
	"python3":    "py",
	"cpp":        "cpp",
	"c":          "c",
	"java":       "java",
	"javascript": "js",
	"typescript": "ts",
	"rust":       "rs",
	"ruby":       "rb",
	"swift":      "swift",
	"kotlin":     "kt",
	"scala":      "scala",
	"csharp":     "cs",
	"php":        "php",
	"bash":       "sh",
	"mysql":      "sql",
	"mssql":      "sql",
	"oraclesql":  "sql",
	"postgresql": "sql",
	"elixir":     "ex",
	"erlang":     "erl",
	"racket":     "rkt",
	"dart":       "dart",
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

// Cache is the on-disk store for scaffolded solution files, rooted at
// os.UserCacheDir()/leetcode-anki. The zero value is usable; NewCache exists
// only to make construction at the wiring site read consistently.
type Cache struct{}

// NewCache returns a Cache backed by os.UserCacheDir().
func NewCache() *Cache { return &Cache{} }

// Scaffold creates the solution file (with the language's starter snippet) if
// it does not already exist. If the file exists, it is left untouched so the
// user can resume work in progress. Returns the resolved file path.
func (c *Cache) Scaffold(titleSlug, langSlug, snippet string) (string, error) {
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

// ScaffoldAttemptTmp writes snippet to a fresh per-attempt directory
// under $TMPDIR and returns the path of the file inside. Review Mode
// uses this to avoid opening the canonical solution.<ext> in $EDITOR.
//
// Each attempt gets its own directory so Go's "all .go files in a
// directory share one package" rule doesn't cause duplicate-symbol
// errors between concurrent attempts (or between this attempt and
// leftover files from a prior session).
func (c *Cache) ScaffoldAttemptTmp(langSlug, snippet string) (string, error) {
	dir, err := os.MkdirTemp("", "leetcode-anki-attempt-*")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "solution."+Ext(langSlug))
	if err := os.WriteFile(path, []byte(snippet), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Read returns the current contents of the solution file at path.
func (c *Cache) Read(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read solution: %w", err)
	}
	return string(b), nil
}

// ExistingPath returns the on-disk path of a previously-written solution for
// this problem+language, or "" if no file exists. Used to detect resumable
// work without scaffolding a fresh file.
func (c *Cache) ExistingPath(titleSlug, langSlug string) string {
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

// SlugsWith returns the set of title slugs that have at least one cached
// solution file. A single ReadDir of the cache root, so callers can stamp
// every row of the lists screen without an os.Stat per row.
func (c *Cache) SlugsWith() (map[string]bool, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(dir, "leetcode-anki")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() || !validSlug.MatchString(e.Name()) {
			continue
		}
		if slugDirHasSolution(filepath.Join(root, e.Name())) {
			out[e.Name()] = true
		}
	}
	return out, nil
}

// HasAny reports whether the slug has at least one cached solution file in
// any language.
func (c *Cache) HasAny(titleSlug string) bool {
	if !validSlug.MatchString(titleSlug) {
		return false
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return false
	}
	return slugDirHasSolution(filepath.Join(dir, "leetcode-anki", titleSlug))
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

// Runner spawns the user's editor on a solution file. The zero value is
// usable; $VISUAL → $EDITOR → "vi" is resolved at Open time, not construction.
type Runner struct{}

// NewRunner returns a Runner that resolves $VISUAL/$EDITOR at Open time.
func NewRunner() *Runner { return &Runner{} }

// Open returns a tea.Cmd that suspends the TUI, runs the user's editor on
// path, and on exit emits an EditorDoneMsg.
func (r *Runner) Open(path string) tea.Cmd {
	ed := resolveEditor()
	cmd := exec.Command(ed, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditorDoneMsg{Path: path, Err: err}
	})
}

func slugDirHasSolution(slugDir string) bool {
	entries, err := os.ReadDir(slugDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "solution.") {
			return true
		}
	}
	return false
}

// ChromaLang maps a LeetCode langSlug to the lexer name expected inside a
// glamour-rendered fenced code block. Slugs not in the table are returned
// unchanged — chroma falls back to plain text on unknown lexers.
func ChromaLang(langSlug string) string {
	if c, ok := chromaByLangSlug[langSlug]; ok {
		return c
	}
	return langSlug
}

var chromaByLangSlug = map[string]string{
	"golang":    "go",
	"python3":   "python",
	"mssql":     "tsql",
	"oraclesql": "plsql",
}

var commentPrefixByLangSlug = map[string]string{
	"golang":     "//",
	"c":          "//",
	"cpp":        "//",
	"java":       "//",
	"javascript": "//",
	"typescript": "//",
	"rust":       "//",
	"swift":      "//",
	"kotlin":     "//",
	"scala":      "//",
	"csharp":     "//",
	"dart":       "//",
	"php":        "//",
	"python":     "#",
	"python3":    "#",
	"ruby":       "#",
	"bash":       "#",
	"elixir":     "#",
	"mysql":      "--",
	"mssql":      "--",
	"oraclesql":  "--",
	"postgresql": "--",
	"erlang":     "%",
	"racket":     ";",
}

// CommentBlock prefixes each line of body with langSlug's line-comment marker.
// Empty body or an unmapped langSlug returns "" so callers can append the
// result unconditionally without producing a comment block of the wrong shape.
func CommentBlock(body, langSlug string) string {
	if body == "" {
		return ""
	}
	prefix, ok := commentPrefixByLangSlug[langSlug]
	if !ok {
		return ""
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = prefix
		} else {
			lines[i] = prefix + " " + line
		}
	}
	return strings.Join(lines, "\n")
}

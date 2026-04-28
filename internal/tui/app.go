package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
	"leetcode-anki/internal/sr"
)

type screen int

const (
	screenLists screen = iota
	screenProblems
	screenProblem
	screenResult
)

// Model is the root Bubble Tea model that owns shared TUI state (auth client,
// solution cache, editor runner, current list/problem/language, in-flight
// cancellation) and routes Update/View to the active screen.
type Model struct {
	client  LeetcodeClient
	cache   SolutionCache
	editor  Editor
	reviews sr.Reviews
	ctx     context.Context

	// cancelInflight cancels the currently in-flight run/submit request when
	// the user presses esc on the result-loading screen, or when the program
	// shuts down. Nil when no request is in flight.
	cancelInflight context.CancelFunc

	width, height int

	screen screen
	err    error

	// load drives the visible loading indicator on the active screen.
	// previewLoad is a separate instance because the side-pane preview
	// can fetch concurrently with a per-screen load (e.g. arrow-keying
	// through the problems list while the list itself is rehydrating).
	load        Indicator
	previewLoad Indicator

	// Lists screen
	lists      list.Model
	listsReady bool

	// reviewMode is a sticky session toggle. When on, list screens render
	// with a Review-Mode indicator and the problems screen filters to
	// Problems currently due. Toggled by 'v' from any list-ish screen.
	reviewMode bool

	// Problems screen
	currentList   leetcode.FavoriteList
	problems      list.Model
	problemsReady bool
	problemIndex  int
	preview       previewState
	solutionSlugs map[string]bool

	// problemsAll is the unfiltered slice loaded for currentList. The
	// problems list view derives its items from this, optionally filtered
	// through dueSlugs, so 'v' can flip Review/Explore without re-fetching.
	problemsAll []Problem

	// dueSlugs is the set of slugs in problemsAll currently due for review.
	// Populated by loadProblemsCmd when reviewMode is on at load time;
	// nil when the load happened in Explore Mode (no SR work was done).
	// Cleared whenever a different list is loaded.
	dueSlugs map[string]bool

	// Problem screen
	currentProblem *leetcode.ProblemDetail
	problem        problemView

	// Result screen
	result resultView
}

func NewModel(ctx context.Context, client LeetcodeClient, cache SolutionCache, ed Editor, reviews sr.Reviews) *Model {
	return &Model{
		client:      client,
		cache:       cache,
		editor:      ed,
		reviews:     reviews,
		ctx:         ctx,
		screen:      screenLists,
		load:        NewIndicator(),
		previewLoad: NewIndicator(),
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.load.Start(KindNeutral, "loading your lists"), loadListsCmd(m.ctx, m.client))
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := m.load.Update(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.previewLoad.Update(msg); handled {
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listH := msg.Height - listsChromeHeight
		if listH < 5 {
			listH = 5
		}
		if m.listsReady {
			m.lists.SetSize(msg.Width, listH)
		}
		if m.problemsReady {
			lw, lh, pw, ph := problemsLayout(msg.Width, msg.Height)
			m.problems.SetSize(lw, lh)
			m.preview.setSize(pw, ph)
		}
		if m.problem.rendered != "" && m.currentProblem != nil {
			_ = m.problem.renderForLayout(m.currentProblem, msg.Width, msg.Height)
		} else {
			m.problem.vp.Width = msg.Width
			m.problem.vp.Height = msg.Height - 5
		}
		return m, nil

	case tea.KeyMsg:
		// Cancel an in-flight run/submit when the user changes their mind. esc
		// during the spinner aborts the request; ctrl+c always quits and also
		// cancels.
		if msg.String() == "esc" && m.load.Active() && (m.load.kind == KindRun || m.load.kind == KindSubmit) {
			if m.cancelInflight != nil {
				m.cancelInflight()
			}
			return m, nil
		}
		if msg.String() == "ctrl+c" {
			if m.cancelInflight != nil {
				m.cancelInflight()
			}
			return m, tea.Quit
		}

	case errMsg:
		m.err = msg.error
		m.load.Stop()
		m.previewLoad.Stop()
		m.clearInflight()
		return m, nil

	case listsLoadedMsg:
		m.load.Stop()
		listH := m.height - listsChromeHeight
		if listH < 5 {
			listH = 20
		}
		w := m.width
		if w < 20 {
			w = 80
		}
		m.lists = newListsList(w, listH, msg.lists)
		m.listsReady = true
		return m, nil

	case problemsLoadedMsg:
		m.load.Stop()
		w := m.width
		if w < 20 {
			w = 80
		}
		h := m.height
		if h < 7 {
			h = 24
		}
		lw, lh, pw, ph := problemsLayout(w, h)
		m.solutionSlugs = msg.solutions
		if m.solutionSlugs == nil {
			m.solutionSlugs = map[string]bool{}
		}
		m.problemsAll = msg.problems
		m.dueSlugs = msg.dueSlugs
		visible := visibleProblems(m.problemsAll, m.reviewMode, m.dueSlugs)
		m.problems = newProblemsList(lw, lh, visible, m.currentList.Name, m.solutionSlugs)
		m.problemsReady = true
		m.problemIndex = 0
		m.preview = previewState{}
		m.preview.setSize(pw, ph)
		m.screen = screenProblems
		return m, syncPreviewCursor(m)

	case previewTickMsg:
		if !m.preview.tickFired(msg.slug) {
			return m, nil
		}
		return m, tea.Batch(m.previewLoad.Start(KindNeutral, "loading preview"), loadPreviewCmd(m.ctx, m.client, msg.slug))

	case previewLoadedMsg:
		m.previewLoad.Stop()
		m.preview.fetchReturned(msg.slug, msg.detail, msg.err)
		return m, nil

	case problemLoadedMsg:
		m.load.Stop()
		m.currentProblem = msg.problem
		w := m.width
		if w < 20 {
			w = 80
		}
		h := m.height - 5
		if h < 5 {
			h = 20
		}
		m.problem = newProblemView(m.cache, w, h)
		status := m.statusFor(msg.problem.TitleSlug)
		hasSolution := m.solutionSlugs[msg.problem.TitleSlug]
		if err := m.problem.setProblem(msg.problem, status, hasSolution, w, m.height); err != nil {
			m.err = err
		}
		m.screen = screenProblem
		return m, nil

	case editor.EditorDoneMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		if m.currentProblem != nil {
			// Only mark the slug as having a local Solution when the
			// editor was opened on the canonical solution.<ext>. Review
			// Mode opens a temp attempt instead — editing it doesn't
			// create or change a canonical Solution on disk.
			if msg.Path != "" && msg.Path == m.problem.solutionPath {
				slug := m.currentProblem.TitleSlug
				if m.solutionSlugs == nil {
					m.solutionSlugs = map[string]bool{}
				}
				m.solutionSlugs[slug] = true
				m.problem.hasSolution = true
				if m.problemsReady {
					for i, it := range m.problems.Items() {
						if pi, ok := it.(problemItem); ok && pi.q.TitleSlug == slug {
							pi.hasSolution = true
							m.problems.SetItem(i, pi)
							break
						}
					}
				}
			}
			if m.problem.rendered != "" {
				_ = m.problem.renderForLayout(m.currentProblem, m.width, m.height)
			}
		}
		return m, nil

	case runResultMsg:
		m.load.Stop()
		m.clearInflight()
		m.result = resultView{kind: resultRun, run: msg.result}
		m.screen = screenResult
		return m, nil

	case submitResultMsg:
		m.load.Stop()
		m.clearInflight()
		m.result = resultView{kind: resultSubmit, submit: msg.result}
		m.screen = screenResult
		if msg.result != nil && msg.result.StatusMsg == "Accepted" && m.currentProblem != nil {
			slug := m.currentProblem.TitleSlug
			m.markSolved(slug)
			// Defer Record to the rating modal so the user's actual grade
			// (1-4) is what gets stored, not the system's "Accepted = Good"
			// guess. Preview errors are swallowed: empty previews render as
			// "—" rather than blocking the modal.
			previews, _ := m.reviews.Preview(m.ctx, slug, time.Now())
			m.result.grade = &gradeModalState{cursor: 2, previews: previews}
		}
		return m, nil
	}

	// Cold loads (KindNeutral) take over the active screen: the user has
	// nothing useful to do until lists / problems / problem-detail arrive.
	// Run / submit (KindRun / KindSubmit) intentionally do NOT swallow
	// input — they render an inline indicator so the user can keep reading
	// the problem and esc-cancel.
	if m.load.Active() && m.load.kind == KindNeutral {
		return m, nil
	}

	switch m.screen {
	case screenLists:
		return updateListsView(m, msg)
	case screenProblems:
		return updateProblemsView(m, msg)
	case screenProblem:
		return updateProblemView(m, msg)
	case screenResult:
		return updateResultView(m, msg)
	}
	return m, nil
}

func (m *Model) View() string {
	if m.err != nil {
		return renderWithBanner(m.viewScreen(), errorStyle.Render("error: "+truncateErr(m.err.Error(), 240)))
	}
	if m.load.Active() && m.load.kind == KindNeutral {
		return m.load.View()
	}
	return m.viewScreen()
}

func (m *Model) viewScreen() string {
	switch m.screen {
	case screenLists:
		return viewListsView(m)
	case screenProblems:
		return viewProblemsView(m)
	case screenProblem:
		return viewProblemView(m)
	case screenResult:
		return viewResultView(m)
	}
	return ""
}

func renderWithBanner(view, banner string) string {
	return banner + "\n" + view
}

func truncateErr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// keyMatch is a small alias so views can read more naturally.
func keyMatch(m tea.KeyMsg, b key.Binding) bool {
	return key.Matches(m, b)
}

// visibleProblems is the projection from the unfiltered list to whatever
// the problems screen should currently show. Explore Mode passes the
// slice through; Review Mode keeps only Problems whose slug is in
// dueSlugs. A nil dueSlugs in Review Mode means the load was made before
// SR data was available — fall back to the full list rather than render
// an empty pane.
func visibleProblems(all []Problem, reviewMode bool, dueSlugs map[string]bool) []Problem {
	if !reviewMode || dueSlugs == nil {
		return all
	}
	out := make([]Problem, 0, len(dueSlugs))
	for _, q := range all {
		if dueSlugs[q.TitleSlug] {
			out = append(out, q)
		}
	}
	return out
}

// markSolved updates the in-memory list and the open problem-view to AC
// after a successful submit. The favorites-list query isn't re-fetched
// during a session, so without this the row stays at its load-time status
// and the local-Solution signal drags the glyph back to "in progress".
func (m *Model) markSolved(slug string) {
	ac := "AC"
	if m.problemsReady {
		for i, it := range m.problems.Items() {
			if pi, ok := it.(problemItem); ok && pi.q.TitleSlug == slug {
				pi.q.Status = &ac
				m.problems.SetItem(i, pi)
				break
			}
		}
	}
	if m.problem.status == nil || !isAccepted(m.problem.status) {
		m.problem.status = &ac
	}
}

// statusFor returns the LeetCode Status for a slug, looked up from the
// currently loaded problems list. The detail screen needs this because
// ProblemDetail itself doesn't carry status — only the list query does.
func (m *Model) statusFor(slug string) *string {
	if !m.problemsReady {
		return nil
	}
	for _, it := range m.problems.Items() {
		if pi, ok := it.(problemItem); ok && pi.q.TitleSlug == slug {
			return pi.q.Status
		}
	}
	return nil
}

// clearInflight releases the cancel function for the most recent run/submit
// request. Called whenever a request settles (success or error) or is cancelled.
func (m *Model) clearInflight() {
	if m.cancelInflight != nil {
		m.cancelInflight()
		m.cancelInflight = nil
	}
}

// Run starts the TUI loop. The provided ctx is the parent context for all
// outbound HTTP requests; cancelling it (e.g. on SIGINT from the parent
// process) will abort any in-flight run/submit.
func Run(ctx context.Context, client LeetcodeClient, cache SolutionCache, ed Editor, reviews sr.Reviews) error {
	m := NewModel(ctx, client, cache, ed, reviews)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	m.clearInflight()
	return err
}

// --- messages ---

type errMsg struct{ error }

type listsLoadedMsg struct {
	lists []leetcode.FavoriteList
}

type problemsLoadedMsg struct {
	problems  []Problem
	solutions map[string]bool
	// dueSlugs is the set of currently-due slugs computed by loadProblemsCmd
	// when reviewMode was on at load time. Nil when the load happened in
	// Explore Mode (no SR fan-out). Used by the Update handler to filter
	// the rendered list and retained on the model so 'v' toggles can
	// re-render without re-fetching.
	dueSlugs map[string]bool
}

type problemLoadedMsg struct {
	problem *leetcode.ProblemDetail
}

type runResultMsg struct {
	result *leetcode.RunResult
}

type submitResultMsg struct {
	result *leetcode.SubmitResult
}

// --- commands ---

func loadListsCmd(parent context.Context, c LeetcodeClient) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, defaultTimeout)
		defer cancel()
		lists, err := c.MyFavoriteLists(ctx)
		if err != nil {
			return errMsg{err}
		}
		return listsLoadedMsg{lists: lists}
	}
}

// loadProblemsCmd fetches the list contents and, when reviewMode is true,
// also computes the set of due slugs in one round-trip. Non-AC slugs
// short-circuit without an SR call so the fan-out is bounded to AC
// Problems in this list. Per-slug Status errors are swallowed (the slug
// is just omitted from dueSlugs) so a single SR failure can't collapse
// the screen.
func loadProblemsCmd(parent context.Context, c LeetcodeClient, cache SolutionCache, slug string, reviewMode bool, reviews sr.Reviews) tea.Cmd {
	return func() tea.Msg {
		timeout := defaultTimeout
		if reviewMode {
			timeout = reviewTimeout
		}
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()
		res, err := c.FavoriteQuestionList(ctx, slug, 0, 500)
		if err != nil {
			return errMsg{err}
		}
		solutions, _ := cache.SlugsWith()

		var dueSlugs map[string]bool
		if reviewMode {
			now := time.Now()
			dueSlugs = map[string]bool{}
			for _, q := range res.Questions {
				if !isAccepted(q.Status) {
					continue
				}
				st, err := reviews.Status(ctx, q.TitleSlug, now)
				if err != nil {
					continue
				}
				if st.Due(now) {
					dueSlugs[q.TitleSlug] = true
				}
			}
		}
		return problemsLoadedMsg{problems: res.Questions, solutions: solutions, dueSlugs: dueSlugs}
	}
}

// deliverProblem synthesizes the problemLoadedMsg path for a problem already
// held in the preview cache, sparing the user a redundant 30s GraphQL fetch.
func deliverProblem(p *leetcode.ProblemDetail) tea.Cmd {
	return func() tea.Msg {
		return problemLoadedMsg{problem: p}
	}
}

func loadProblemCmd(parent context.Context, c LeetcodeClient, titleSlug string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, defaultTimeout)
		defer cancel()
		p, err := c.ProblemDetail(ctx, titleSlug)
		if err != nil {
			return errMsg{err}
		}
		return problemLoadedMsg{problem: p}
	}
}

// runCodeCmd reads the solution off disk and runs it server-side.
//
// The returned cmd is paired with cancelFn: the model stores cancelFn so that
// pressing esc during the run can cancel the in-flight HTTP request, instead
// of the goroutine continuing to run and eventually clobbering model state
// with a stale runResultMsg.
func runCodeCmd(parent context.Context, c LeetcodeClient, cache SolutionCache, p *leetcode.ProblemDetail, langSlug, path string) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, submitTimeout)
	cmd := func() tea.Msg {
		code, err := cache.Read(path)
		if err != nil {
			return errMsg{err}
		}
		res, err := c.InterpretSolution(ctx, p.TitleSlug, langSlug, p.QuestionID, code, p.ExampleTestcases, p.MetaData)
		if err != nil {
			return errMsg{err}
		}
		return runResultMsg{result: res}
	}
	return cmd, cancel
}

func submitCodeCmd(parent context.Context, c LeetcodeClient, cache SolutionCache, p *leetcode.ProblemDetail, langSlug, path string) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, submitTimeout)
	cmd := func() tea.Msg {
		code, err := cache.Read(path)
		if err != nil {
			return errMsg{err}
		}
		res, err := c.Submit(ctx, p.TitleSlug, langSlug, p.QuestionID, code)
		if err != nil {
			return errMsg{err}
		}
		return submitResultMsg{result: res}
	}
	return cmd, cancel
}

// --- timeouts ---

const (
	defaultTimeout      = 30 * time.Second
	submitTimeout       = 120 * time.Second
	reviewTimeout       = 120 * time.Second
	previewFetchTimeout = 15 * time.Second
	previewDebounce     = 220 * time.Millisecond
)

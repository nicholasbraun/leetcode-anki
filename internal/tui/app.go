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
	// through session, so 'v' can flip Review/Explore without re-fetching.
	problemsAll []Problem

	// session is the Review Mode queue for currentList: the ordered set of
	// due-then-new SessionItems plus uncapped counts. Populated by
	// loadProblemsCmd when reviewMode is on at load time; nil when the
	// load happened in Explore Mode (no SR work was done). Cleared
	// whenever a different list is loaded.
	session *sr.Session

	// reviewDue / reviewNew cap how many due Items and new Items appear
	// in a Review Mode queue. Threaded through loadProblemsCmd into
	// reviews.Session. Defaults set in NewModel; CLI flags override.
	reviewDue int
	reviewNew int

	// userIsPremium reflects the LeetCode account's subscription. False
	// is the safe default (zero value) — when set, loadProblemsCmd stops
	// stripping PaidOnly Problems from the Review-Mode slug list. The
	// flag flows from leetcode.Verify through tui.Run; tests set it
	// directly on the Model when relevant.
	userIsPremium bool

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
		reviewDue:   defaultReviewDue,
		reviewNew:   defaultReviewNew,
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
		m.session = msg.session
		visible := visibleProblems(m.problemsAll, m.reviewMode, m.session)
		m.problems = newProblemsList(lw, lh, visible, m.currentList.Name, m.solutionSlugs, sessionBadges(m.session, time.Now()))
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
// slice through; Review Mode walks session.Items so the visible order
// matches the SR-determined "due first, then new" queue. A nil session
// in Review Mode means the load was made before SR data was available —
// fall back to the full list rather than render an empty pane.
func visibleProblems(all []Problem, reviewMode bool, session *sr.Session) []Problem {
	if !reviewMode || session == nil {
		return all
	}
	bySlug := make(map[string]Problem, len(all))
	for _, q := range all {
		bySlug[q.TitleSlug] = q
	}
	out := make([]Problem, 0, len(session.Items))
	for _, it := range session.Items {
		if q, ok := bySlug[it.TitleSlug]; ok {
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
// process) will abort any in-flight run/submit. reviewDue / reviewNew
// override the default Review Mode bucket caps; the caller is expected
// to clamp negatives to zero before invoking. userIsPremium is the
// LeetCode account's subscription status — when false, paid Problems
// are stripped from Review-Mode recommendations.
func Run(ctx context.Context, client LeetcodeClient, cache SolutionCache, ed Editor, reviews sr.Reviews, reviewDue, reviewNew int, userIsPremium bool) error {
	m := NewModel(ctx, client, cache, ed, reviews)
	m.reviewDue = reviewDue
	m.reviewNew = reviewNew
	m.userIsPremium = userIsPremium
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
	// session is the Review Mode queue computed by loadProblemsCmd when
	// reviewMode was on at load time. Nil when the load happened in
	// Explore Mode (no SR fan-out). Retained on the Model so 'v' toggles
	// can re-render without re-fetching.
	session *sr.Session
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
// also computes the Review Mode queue in one round-trip via reviews.Session.
// Free users (userIsPremium == false) get PaidOnly Problems stripped from
// the slug list before SR sees it, so the new bucket can't recommend
// Problems they can't open. Session errors degrade to a nil queue so a
// single SR failure can't collapse the screen — the user lands on the
// unfiltered list.
func loadProblemsCmd(parent context.Context, c LeetcodeClient, cache SolutionCache, slug string, reviewMode bool, dueLimit, newLimit int, userIsPremium bool, reviews sr.Reviews) tea.Cmd {
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

		var session *sr.Session
		if reviewMode {
			slugs := make([]string, 0, len(res.Questions))
			for _, q := range res.Questions {
				if q.PaidOnly && !userIsPremium {
					continue
				}
				slugs = append(slugs, q.TitleSlug)
			}
			s, err := reviews.Session(ctx, sr.SessionConfig{
				Slugs:  slugs,
				MaxDue: dueLimit,
				MaxNew: newLimit,
			}, time.Now())
			if err == nil {
				session = &s
			}
		}
		return problemsLoadedMsg{problems: res.Questions, solutions: solutions, session: session}
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

// Defaults for Review Mode bucket caps. Override via --review-due /
// --review-new flags (wired in cmd/leetcode-anki/main.go).
const (
	defaultReviewDue = 2
	defaultReviewNew = 1
)

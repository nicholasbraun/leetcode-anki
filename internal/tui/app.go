package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"leetcode-anki/internal/editor"
	"leetcode-anki/internal/leetcode"
)

var _ LeetcodeClient = (*leetcode.Client)(nil)

type screen int

const (
	screenLists screen = iota
	screenProblems
	screenProblem
	screenResult
)

// Model is the root Bubble Tea model that owns shared TUI state (auth client,
// current list/problem/language, in-flight cancellation) and routes Update/View
// to the active screen.
type Model struct {
	client LeetcodeClient
	ctx    context.Context

	// cancelInflight cancels the currently in-flight run/submit request when
	// the user presses esc on the result-loading screen, or when the program
	// shuts down. Nil when no request is in flight.
	cancelInflight context.CancelFunc

	width, height int

	screen screen
	err    error

	// Lists screen
	lists        list.Model
	listsReady   bool
	listsLoading bool

	// Problems screen
	currentList     leetcode.FavoriteList
	problems        list.Model
	problemsReady   bool
	problemsLoading bool
	problemIndex    int
	preview         previewState

	// Problem screen
	currentProblem *leetcode.ProblemDetail
	problem        problemView
	problemLoading bool

	// Run/submit
	runLoading    bool
	submitLoading bool

	// Result screen
	result resultView
}

func NewModel(ctx context.Context, client LeetcodeClient) *Model {
	return &Model{
		client: client,
		ctx:    ctx,
		screen: screenLists,
	}
}

func (m *Model) Init() tea.Cmd {
	m.listsLoading = true
	return loadListsCmd(m.ctx, m.client)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listH := msg.Height - 2
		if m.listsReady {
			m.lists.SetSize(msg.Width, listH)
		}
		if m.problemsReady {
			lw, lh, pw, ph := problemsLayout(msg.Width, msg.Height)
			m.problems.SetSize(lw, lh)
			m.preview.setSize(pw, ph)
		}
		m.problem.vp.Width = msg.Width
		m.problem.vp.Height = msg.Height - 5
		if m.problem.rendered != "" && m.currentProblem != nil {
			_ = m.problem.setProblem(m.currentProblem, msg.Width)
		}
		return m, nil

	case tea.KeyMsg:
		// Cancel an in-flight run/submit when the user changes their mind. esc
		// during the spinner aborts the request; ctrl+c always quits and also
		// cancels.
		if msg.String() == "esc" && (m.runLoading || m.submitLoading) {
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
		m.listsLoading = false
		m.problemsLoading = false
		m.problemLoading = false
		m.runLoading = false
		m.submitLoading = false
		m.clearInflight()
		return m, nil

	case listsLoadedMsg:
		m.listsLoading = false
		listH := m.height - 2
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
		m.problemsLoading = false
		w := m.width
		if w < 20 {
			w = 80
		}
		h := m.height
		if h < 7 {
			h = 24
		}
		lw, lh, pw, ph := problemsLayout(w, h)
		m.problems = newProblemsList(lw, lh, msg.questions, m.currentList.Name)
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
		return m, loadPreviewCmd(m.ctx, m.client, msg.slug)

	case previewLoadedMsg:
		m.preview.fetchReturned(msg.slug, msg.detail, msg.err)
		return m, nil

	case problemLoadedMsg:
		m.problemLoading = false
		m.currentProblem = msg.problem
		w := m.width
		if w < 20 {
			w = 80
		}
		h := m.height - 5
		if h < 5 {
			h = 20
		}
		m.problem = newProblemView(w, h)
		if err := m.problem.setProblem(msg.problem, w); err != nil {
			m.err = err
		}
		m.screen = screenProblem
		return m, nil

	case editor.EditorDoneMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, nil

	case runResultMsg:
		m.runLoading = false
		m.clearInflight()
		m.result = resultView{kind: resultRun, run: msg.result}
		m.screen = screenResult
		return m, nil

	case submitResultMsg:
		m.submitLoading = false
		m.clearInflight()
		m.result = resultView{kind: resultSubmit, submit: msg.result}
		m.screen = screenResult
		return m, nil
	}

	switch m.screen {
	case screenLists:
		if m.listsLoading {
			return m, nil
		}
		return updateListsView(m, msg)
	case screenProblems:
		if m.problemsLoading {
			return m, nil
		}
		return updateProblemsView(m, msg)
	case screenProblem:
		if m.problemLoading {
			return m, nil
		}
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
	if m.listsLoading {
		return loadingView("loading your lists...")
	}
	if m.problemsLoading {
		return loadingView("loading problems...")
	}
	if m.problemLoading {
		return loadingView("loading problem...")
	}
	if m.runLoading {
		return loadingView("running on LeetCode...")
	}
	if m.submitLoading {
		return loadingView("submitting to LeetCode...")
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

func loadingView(msg string) string {
	return lipgloss.NewStyle().Padding(1, 2).Render(msg)
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
func Run(ctx context.Context, client LeetcodeClient) error {
	m := NewModel(ctx, client)
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
	questions []leetcode.Question
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

func loadProblemsCmd(parent context.Context, c LeetcodeClient, slug string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, defaultTimeout)
		defer cancel()
		res, err := c.FavoriteQuestionList(ctx, slug, 0, 500)
		if err != nil {
			return errMsg{err}
		}
		return problemsLoadedMsg{questions: res.Questions}
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
		p, err := c.Question(ctx, titleSlug)
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
func runCodeCmd(parent context.Context, c LeetcodeClient, p *leetcode.ProblemDetail, langSlug, path string) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, submitTimeout)
	cmd := func() tea.Msg {
		code, err := editor.ReadSolution(path)
		if err != nil {
			return errMsg{err}
		}
		res, err := c.InterpretSolution(ctx, p.TitleSlug, langSlug, p.QuestionID, code, p.ExampleTestcases)
		if err != nil {
			return errMsg{err}
		}
		return runResultMsg{result: res}
	}
	return cmd, cancel
}

func submitCodeCmd(parent context.Context, c LeetcodeClient, p *leetcode.ProblemDetail, langSlug, path string) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, submitTimeout)
	cmd := func() tea.Msg {
		code, err := editor.ReadSolution(path)
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
	previewFetchTimeout = 15 * time.Second
	previewDebounce     = 220 * time.Millisecond
)

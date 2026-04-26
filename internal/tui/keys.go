package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
	Edit     key.Binding
	Run      key.Binding
	Submit   key.Binding
	NextLang key.Binding
	NextProb key.Binding
	PrevProb key.Binding
	Help     key.Binding
}

var keys = keyMap{
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Back:     key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Run:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "run")),
	Submit:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "submit")),
	NextLang: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "language")),
	NextProb: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next problem")),
	PrevProb: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev problem")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

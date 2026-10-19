package stats

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	NextTab key.Binding
	PrevTab key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Tab3    key.Binding
	Tab4    key.Binding
	Up      key.Binding
	Down    key.Binding
	Toggle  key.Binding
	Delete  key.Binding
	Confirm key.Binding
	Cancel  key.Binding
	Quit    key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.NextTab,
		k.Up,
		k.Down,
		k.Toggle,
		k.Delete,
		k.Quit,
	}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

var Keys = KeyMap{
	NextTab: key.NewBinding(
		key.WithKeys("tab", "right", "l"),
		key.WithHelp("tab/→", "switch view"),
	),
	PrevTab: key.NewBinding(
		key.WithKeys("shift+tab", "left", "h"),
		key.WithHelp("←", "prev view"),
	),
	Tab1: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1-4", "views"),
	),
	Tab2: key.NewBinding(
		key.WithKeys("2"),
	),
	Tab3: key.NewBinding(
		key.WithKeys("3"),
	),
	Tab4: key.NewBinding(
		key.WithKeys("4"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Toggle: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter", "expand/collapse"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "x", "delete", "backspace"),
		key.WithHelp("d", "delete"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "yes"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "no"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q", "quit"),
	),
}

var OverviewKeys = KeyMap{
	NextTab: Keys.NextTab,
	Tab1:    Keys.Tab1,
	Quit:    Keys.Quit,
}

var OverviewTitleKeys = KeyMap{
	NextTab: Keys.NextTab,
	Up: key.NewBinding(
		key.WithKeys("up", "k", "down", "j"),
		key.WithHelp("↑/↓", "filter title"),
	),
	Quit: Keys.Quit,
}

var SessionsKeys = KeyMap{
	NextTab: Keys.NextTab,
	Up:      Keys.Up,
	Down:    Keys.Down,
	Delete:  Keys.Delete,
	Quit:    Keys.Quit,
}

var GroupedKeys = KeyMap{
	NextTab: Keys.NextTab,
	Up:      Keys.Up,
	Down:    Keys.Down,
	Toggle:  Keys.Toggle,
	Delete:  Keys.Delete,
	Quit:    Keys.Quit,
}

var ConfirmDeleteKeys = KeyMap{
	Confirm: Keys.Confirm,
	Cancel:  Keys.Cancel,
	NextTab: key.NewBinding(
		key.WithKeys("tab", "left", "right", "h", "l"),
		key.WithHelp("tab", "toggle"),
	),
	Toggle: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "submit"),
	),
	Quit: Keys.Quit,
}

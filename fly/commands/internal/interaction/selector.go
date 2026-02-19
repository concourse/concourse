package interaction

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ list.Item = Item{}

type Item struct {
	Display string
	Value   any
}

func (i Item) FilterValue() string { return i.Display }

type selectModel struct {
	prompt string
	list   list.Model
	choice any
}

func (s selectModel) Choice() any         { return s.choice }
func (s selectModel) UserCancelled() bool { return s.UserCancelled() }

var blankStyle = lipgloss.NewStyle()

func Select(prompt string, items []list.Item) (any, error) {
	l := list.New(items, itemDelegate{}, 20, 15)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowFilter(true)
	l.FilterInput.PromptStyle = blankStyle
	l.FilterInput.Cursor.Style = blankStyle
	l.Help.Styles.ShortKey = blankStyle.Bold(true)
	l.Help.Styles.ShortDesc = blankStyle

	l.KeyMap.Quit = key.NewBinding(
		key.WithKeys("esc", "q"),
		key.WithHelp("[esc/q]", "quit"))
	l.KeyMap.CursorUp.SetHelp("[↑/k]", "up")
	l.KeyMap.CursorDown.SetHelp("[↓/j]", "down")
	l.KeyMap.Filter.SetHelp("[/]", "filter")
	l.KeyMap.GoToEnd.Unbind()
	l.KeyMap.GoToStart.Unbind()
	l.KeyMap.ShowFullHelp.Unbind()

	done, err := tea.NewProgram(selectModel{prompt: prompt, list: l},
		tea.WithInput(checkStdin())).Run()
	if err != nil {
		return nil, err
	}

	choice, ok := done.(selectModel)
	if ok {
		return choice.Choice(), nil
	}
	return nil, errors.New("unknown model returned by bubbletea")
}

func (s selectModel) Init() tea.Cmd {
	return nil
}

func (s selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.list.SetWidth(msg.Width)
		return s, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "esc", "ctrl+c", "ctrl+d":
			switch s.list.FilterState() {
			case list.Filtering, list.FilterApplied:
				// do nothing, let the list model handle it
			default:
				return s, tea.Quit
			}

		case "enter":
			switch s.list.FilterState() {
			case list.Filtering:
				s.list.SetFilterState(list.FilterApplied)
			default:
				i, ok := s.list.SelectedItem().(Item)
				if ok {
					s.choice = i.Value
				}
				return s, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

var titleStyle = lipgloss.NewStyle().MarginLeft(2)

func (s selectModel) View() string {
	finalView := strings.Builder{}
	finalView.WriteString("\n")
	if s.list.FilterState() != list.Filtering {
		finalView.WriteString(titleStyle.Render(fmt.Sprintf("%s:\n", s.prompt)))
	}

	finalView.WriteString(s.list.View())
	return finalView.String()
}

var _ list.ItemDelegate = itemDelegate{}

type itemDelegate struct{}

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(1).Foreground(lipgloss.Color("12"))
)

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	if index == m.Index() {
		fmt.Fprint(w, selectedItemStyle.Render("> ", i.Display))
		return
	}
	fmt.Fprint(w, itemStyle.Render(i.Display))
}

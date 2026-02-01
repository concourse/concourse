package interaction

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Asks a user to confirm an action before taking it. Accepts all variants of
// y/n/yes/no and takes a custom prompt. Defaults to "No".
func Confirm(prompt string) (bool, error) {
	i := textinput.New()
	i.Prompt = fmt.Sprintf("%s [yN]: ", prompt)
	i.PromptStyle = lipgloss.NewStyle()
	i.Focus()
	i.CharLimit = 3
	i.Width = 3

	m, err := tea.NewProgram(confirmModel{input: i},
		tea.WithInput(checkStdin())).Run()
	if err != nil {
		return false, err
	}
	final, ok := m.(confirmModel)
	if ok {
		return final.confirmation, nil
	}
	return false, errors.New("unknown model returned by bubbletea")
}

var _ tea.Model = confirmModel{}

type confirmModel struct {
	input        textinput.Model
	confirmation bool
	unknownInput bool
}

func (c confirmModel) Init() tea.Cmd {
	return textinput.Blink
}

func (c confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			switch strings.ToLower(c.input.Value()) {
			case "y", "yes":
				c.confirmation = true
				return c, tea.Quit
			case "n", "no":
				c.confirmation = false
				return c, tea.Quit
			default:
				c.unknownInput = true
				c.input.SetValue("")
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			c.confirmation = false
			return c, tea.Quit
		}
	}

	c.input, cmd = c.input.Update(msg)
	return c, cmd
}

func (c confirmModel) View() string {
	if c.unknownInput {
		return fmt.Sprintf("%s\n%s",
			c.input.View(),
			"Unknow input. Please enter y/yes/n/no.")
	}
	return c.input.View()
}

package interaction

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Asks a user to confirm an action before taking it. Accepts all variants of
// y/n/yes/no and takes a custom prompt. Defaults to "No".
func Confirm(prompt string) (bool, error) {
	i := textinput.New()
	i.Prompt = fmt.Sprintf("%s [yN]: ", prompt)
	s := i.Styles()
	s.Focused.Prompt = lipgloss.NewStyle()
	i.SetStyles(s)
	i.Focus()
	i.CharLimit = 3
	i.SetWidth(3)

	m, err := tea.NewProgram(confirmModel{input: i},
		// set initial window size for tests
		tea.WithWindowSize(80, 20),
		tea.WithInput(os.Stdin),
	).Run()
	if err != nil {
		fmt.Println(err.Error())
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
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEnter:
			switch strings.ToLower(c.input.Value()) {
			case "y", "yes":
				c.confirmation = true
				return c, tea.Quit
			case "n", "no", "":
				c.confirmation = false
				return c, tea.Quit
			default:
				c.unknownInput = true
				c.input.SetValue("")
			}
		case tea.KeyEscape:
			c.confirmation = false
			return c, tea.Quit
		default:
			if msg.String() == "ctrl+c" {
				c.confirmation = false
				return c, tea.Quit
			}
		}
	}

	c.input, cmd = c.input.Update(msg)
	return c, cmd
}

func (c confirmModel) View() tea.View {
	if c.unknownInput {
		return tea.NewView(fmt.Sprintf("%s\n%s",
			c.input.View(),
			"Unknown input. Please enter y/yes/n/no."))
	}
	return tea.NewView(c.input.View())
}

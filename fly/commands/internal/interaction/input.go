package interaction

import (
	"errors"
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Prompts a user to enter a string and returns the entered string
func Input(prompt string, isSensitive bool) (string, error) {
	prg := InputProgram(prompt, isSensitive)
	m, err := prg.Run()
	if err != nil {
		return "", err
	}
	final, ok := m.(InputModel)
	if ok {
		return final.output, nil
	}
	return "", errors.New("unknown model returned by bubbletea")
}

func InputProgram(prompt string, isSensitive bool) *tea.Program {
	i := textinput.New()
	i.Prompt = fmt.Sprintf("%s: ", prompt)
	s := i.Styles()
	s.Focused.Prompt = lipgloss.NewStyle()
	i.SetStyles(s)
	i.Focus()
	if isSensitive {
		i.EchoMode = textinput.EchoNone
	}
	return tea.NewProgram(InputModel{input: i},
		// set initial window size for tests
		tea.WithWindowSize(80, 20),
		tea.WithInput(Stdin()),
	)
}

var _ tea.Model = InputModel{}

type InputModel struct {
	input  textinput.Model
	output string
}

func (i InputModel) Output() string {
	return i.output
}

func (i InputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (i InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEnter:
			i.output = i.input.Value()
			return i, tea.Quit
		case tea.KeyEscape:
			return i, tea.Quit
		default:
			if msg.String() == "ctrl+c" {
				return i, tea.Quit
			}
		}
	}

	i.input, cmd = i.input.Update(msg)
	return i, cmd
}

func (i InputModel) View() tea.View {
	return tea.NewView(i.input.View())
}

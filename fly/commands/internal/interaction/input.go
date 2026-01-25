package interaction

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	i.PromptStyle = lipgloss.NewStyle()
	i.Focus()
	if isSensitive {
		i.EchoMode = textinput.EchoPassword
	}
	return tea.NewProgram(InputModel{input: i}, tea.WithInput(os.Stdin))
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
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			i.output = i.input.Value()
			return i, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			return i, tea.Quit
		}
	}

	i.input, cmd = i.input.Update(msg)
	return i, cmd
}

func (i InputModel) View() string {
	return i.input.View()
}

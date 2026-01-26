package interaction

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Prompts a user to enter a bearer token
func TokenProgram() *tea.Program {
	i := textinput.New()
	i.Prompt = "or enter token manually: "
	i.PromptStyle = lipgloss.NewStyle()
	i.EchoMode = textinput.EchoPassword
	i.Focus()
	return tea.NewProgram(TokenModel{input: i}, tea.WithInput(os.Stdin))
}

var _ tea.Model = TokenModel{}

type TokenModel struct {
	input          textinput.Model
	malformedToken bool
	token          string
}

func (i TokenModel) Token() string {
	return i.token
}

func (i TokenModel) Init() tea.Cmd {
	return textinput.Blink
}

func (i TokenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			token := i.input.Value()
			parts := strings.Split(token, " ")
			if len(parts) != 2 {
				i.malformedToken = true
				i.input.SetValue("")
			} else {
				i.token = token
				i.malformedToken = false
				return i, tea.Quit
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			return i, tea.Quit
		}
	}

	i.input, cmd = i.input.Update(msg)
	return i, cmd
}

func (i TokenModel) View() string {
	if i.malformedToken {
		return fmt.Sprintf("%s\n%s", i.input.View(), "token must be of the format 'TYPE VALUE', e.g. 'Bearer ...'")
	}
	return i.input.View()
}

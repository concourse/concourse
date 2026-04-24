package interaction

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Prompts a user to enter a bearer token
func TokenProgram() *tea.Program {
	i := textinput.New()
	i.Prompt = "or enter token manually (input hidden): "
	s := i.Styles()
	s.Focused.Prompt = lipgloss.NewStyle()
	i.SetStyles(s)
	i.EchoMode = textinput.EchoNone
	i.Focus()
	return tea.NewProgram(TokenModel{input: i},
		// set initial window size for tests
		tea.WithWindowSize(80, 20),
		tea.WithInput(os.Stdin),
	)
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
	case tea.KeyPressMsg:
		switch msg.Code {
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

func (i TokenModel) View() tea.View {
	if i.malformedToken {
		return tea.NewView(fmt.Sprintf("%s\n%s", i.input.View(), "token must be of the format 'TYPE VALUE', e.g. 'Bearer ...'"))
	}
	return tea.NewView(i.input.View())
}

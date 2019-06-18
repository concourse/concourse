package commands

import "fmt"

type CompletionCommand struct {
	Shell string `long:"shell" required:"true" choice:"bash" choice:"zsh"` // add more choices later
}

// credits:
// https://godoc.org/github.com/jessevdk/go-flags#hdr-Completion
// https://github.com/concourse/concourse/issues/1309#issuecomment-452893900
const bashCompletionSnippet = `_fly_compl() {
	args=("${COMP_WORDS[@]:1:$COMP_CWORD}")
	local IFS=$'\n'
	COMPREPLY=($(GO_FLAGS_COMPLETION=1 ${COMP_WORDS[0]} "${args[@]}"))
	return 0
}
complete -F _fly_compl fly
`

// initial implemenation just using bashcompinit
const zshCompletionSnippet = `autoload -Uz compinit && compinit
autoload -Uz bashcompinit && bashcompinit
` + bashCompletionSnippet

func (command *CompletionCommand) Execute([]string) error {
	switch command.Shell {
	case "bash":
		_, err := fmt.Print(bashCompletionSnippet)
		return err
	case "zsh":
		_, err := fmt.Print(zshCompletionSnippet)
		return err
	default:
		// this should be unreachable
		return fmt.Errorf("unknown shell %s", command.Shell)
	}
}

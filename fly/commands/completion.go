package commands

import "fmt"

type CompletionCommand struct {
	Shell string `long:"shell" required:"true" choice:"bash"` // add more choices later
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

func (*CompletionCommand) Execute([]string) error {
	_, err := fmt.Print(bashCompletionSnippet)
	return err
}

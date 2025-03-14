package commands

import (
	"fmt"
	"reflect"
	"strings"
)

type CompletionCommand struct {
	Shell string `long:"shell" required:"true" choice:"bash" choice:"zsh" choice:"fish"` // add more choices later
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

func fishCompletionSnippetHelper(snippet string, prefix string, commandType reflect.Type) string {
	for i := 0; i < commandType.NumField(); i++ {
		field := commandType.Field(i)
		var tags = field.Tag
		var template = "complete -c fly"

		var command = tags.Get("command")
		var alias = tags.Get("alias")
		var long = tags.Get("long")
		var short = tags.Get("short")
		var description = tags.Get("description")

		if command != "" {
			template += fmt.Sprintf(" -n __fish_use_subcommand -a \"%s\"", command)
		}

		if prefix != "" {
			template += fmt.Sprintf(" -n \"__fish_seen_subcommand_from %s\"", strings.TrimSpace(prefix))
		}

		if description != "" {
			if alias != "" {
				template += fmt.Sprintf(" -d \"%s (alias: %s)\"", description, alias)
			} else {
				template += fmt.Sprintf(" -d \"%s\"", description)
			}
		}

		if long != "" {
			template += fmt.Sprintf(" -l \"%s\"", long)
		}

		if short != "" {
			template += fmt.Sprintf(" -s \"%s\"", short)
		}

		snippet += template + "\n"

		// A subcommand is found, recursion begins.
		if command != "" {
			// Ensure there's exactly one space between commands in the prefix
			newPrefix := strings.TrimSpace(prefix) + " " + command

			// Make sure we only recurse into struct fields
			fieldType := field.Type

			// Skip func() types (like the Version field)
			if fieldType.Kind() == reflect.Func {
				continue
			}

			// Handle pointer types
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			// Only recurse into structs
			if fieldType.Kind() == reflect.Struct {
				snippet = fishCompletionSnippetHelper(snippet, newPrefix, fieldType)
			}
		}
	}

	return snippet
}

var fishCompletionSnippet = fishCompletionSnippetHelper("", "", reflect.TypeOf(Fly))

// Initial implementation just using bashcompinit
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
	case "fish":
		_, err := fmt.Print(fishCompletionSnippet)
		return err
	default:
		// This should be unreachable
		return fmt.Errorf("unknown shell %s", command.Shell)
	}
}

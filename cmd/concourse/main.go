package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
)

// FLAGS NEEDED FOR CLUSTER:
// --peer-ip for --peer-url and local worker registration
//
// --session-signing-key so all ATCs trust each other's tokens
//
// TODO: worker registration
// TODO: baggageclaim
// TODO: fly cli downloads
func main() {
	var cmd ConcourseCommand

	parser := flags.NewParser(&cmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"

	flags "github.com/jessevdk/go-flags"
)

func main() {
	var reaperCmd ReaperCmd

	parser := flags.NewParser(&reaperCmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	reaperCmd.Execute()
}

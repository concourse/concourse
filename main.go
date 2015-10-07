package main

import (
	"fmt"
	"os"

	"github.com/concourse/fly/commands"
)

func main() {
	_, err := commands.Parser.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

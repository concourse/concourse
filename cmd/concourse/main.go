package main

import (
	"fmt"
	"os"
)

func main() {
	if err := concourseCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

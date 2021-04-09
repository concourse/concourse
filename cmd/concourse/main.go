package main

import (
	"fmt"
	"os"
)

func main() {
	err := ConcourseCommand.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

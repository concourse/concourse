package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
)

func main() {
	cmd := &ATCCommand{}

	parser := flags.NewParser(cmd, flags.Default)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	running := ifrit.Invoke(cmd)

	err = <-running.Wait()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"

	"github.com/concourse/concourse/tsa/tsacmd"
)

func main() {
	err := tsacmd.TSACommand.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

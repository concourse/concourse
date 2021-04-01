package main

import (
	_ "net/http/pprof"

	"github.com/concourse/concourse/tsa/tsacmd"
)

func main() {
	err := tsacmd.TSACommand.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}

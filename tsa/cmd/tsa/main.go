package main

import (
	_ "net/http/pprof"

	"github.com/concourse/concourse/tsa/tsacmd"
)

func main() {
	tsacmd.TSACommand.Execute()
}

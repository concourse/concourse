package worker

import (
	"os"
)

var drainSignals = []os.Signal{}

func isLand(sig os.Signal) bool {
	return false
}

func isRetire(sig os.Signal) bool {
	return false
}

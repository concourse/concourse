package drain

import (
	"os"
)

var Signals = []os.Signal{}

func IsLand(sig os.Signal) bool {
	return false
}

func IsRetire(sig os.Signal) bool {
	return false
}

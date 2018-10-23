package drain

import (
	"os"
	"syscall"
)

var Signals = []os.Signal{}

func IsLand(sig os.Signal) bool {
	return false
}

func IsRetire(sig os.Signal) bool {
	return false
}

func IsStop(sig os.Signal) bool {
	return sig == syscall.SIGTERM || sig == syscall.SIGINT
}

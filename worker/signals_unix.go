// +build !windows

package worker

import (
	"os"
	"syscall"
)

var drainSignals = []os.Signal{
	syscall.SIGUSR1,
	syscall.SIGUSR2,
}

func isLand(sig os.Signal) bool {
	return sig == syscall.SIGUSR1
}

func isRetire(sig os.Signal) bool {
	return sig == syscall.SIGUSR2
}

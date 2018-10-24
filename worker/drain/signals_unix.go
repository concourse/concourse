// +build !windows

package drain

import (
	"os"
	"syscall"
)

var Signals = []os.Signal{
	syscall.SIGUSR1,
	syscall.SIGUSR2,
}

func IsLand(sig os.Signal) bool {
	return sig == syscall.SIGUSR1
}

func IsRetire(sig os.Signal) bool {
	return sig == syscall.SIGUSR2
}

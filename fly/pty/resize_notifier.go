//+build !windows

package pty

import (
	"os"
	"os/signal"
	"syscall"
)

func ResizeNotifier() <-chan os.Signal {
	resized := make(chan os.Signal, 10)
	signal.Notify(resized, syscall.SIGWINCH)
	return resized
}

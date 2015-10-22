//+build windows

package pty

import "os"

func ResizeNotifier() <-chan os.Signal {
	return nil
}

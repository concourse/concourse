//go:build linux || freebsd || solaris || openbsd
// +build linux freebsd solaris openbsd

// Package kernel provides helper function to get, parse and compare kernel
// versions for different platforms.
package kernel

import (
	"golang.org/x/sys/unix"
)

// GetKernelVersion gets the current kernel version.
func GetKernelVersion() (*VersionInfo, error) {
	uts, err := uname()
	if err != nil {
		return nil, err
	}

	// Remove the \x00 from the release for Atoi to parse correctly
	return ParseRelease(unix.ByteSliceToString(uts.Release[:]))
}

// CheckKernelVersion checks if current kernel is newer than (or equal to)
// the given version.
func CheckKernelVersion(k, major, minor int) (bool, error) {
	if v, err := GetKernelVersion(); err != nil {
		return false, err
	} else {
		if CompareKernelVersion(*v, VersionInfo{Kernel: k, Major: major, Minor: minor}) < 0 {
			return false, nil
		}
	}

	return true, nil
}

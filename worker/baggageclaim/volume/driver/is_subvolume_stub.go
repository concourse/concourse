//go:build !linux
// +build !linux

package driver

func isSubvolume(_ string) (bool, error) {
	return false, nil
}

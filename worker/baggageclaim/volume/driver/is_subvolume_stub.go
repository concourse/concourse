//go:build !linux
// +build !linux

package driver

func isSubvolume(p string) (bool, error) {
	return false, nil
}

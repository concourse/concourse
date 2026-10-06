//go:build windows

package runtime

import (
	"fmt"
	"os"

	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	offreg       = windows.NewLazySystemDLL("offreg.dll")
	orOpenHive   = offreg.NewProc("OROpenHive")
	orMergeHives = offreg.NewProc("ORMergeHives")
	orSaveHive   = offreg.NewProc("ORSaveHive")
	orCloseHive  = offreg.NewProc("ORCloseHive")
)

type orHKey uintptr

func openHive(path string) (orHKey, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var hive orHKey
	ret, _, _ := orOpenHive.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&hive)),
	)
	if ret != 0 {
		return 0, fmt.Errorf("OROpenHive %s: %w", path, windows.Errno(ret))
	}

	return hive, nil
}

func closeHive(hive orHKey) {
	_, _, _ = orCloseHive.Call(uintptr(hive))
}

// mergeHive merges a differencing hive onto a base hive, replacing the base
// hive file with the merged result.
func mergeHive(basePath, deltaPath string) error {
	base, err := openHive(basePath)
	if err != nil {
		return err
	}
	defer closeHive(base)

	delta, err := openHive(deltaPath)
	if err != nil {
		return err
	}
	defer closeHive(delta)

	hives := []orHKey{base, delta}

	var merged orHKey
	ret, _, _ := orMergeHives.Call(
		uintptr(unsafe.Pointer(&hives[0])),
		uintptr(len(hives)),
		uintptr(unsafe.Pointer(&merged)),
	)
	if ret != 0 {
		return fmt.Errorf("ORMergeHives: %w", windows.Errno(ret))
	}
	defer closeHive(merged)

	version := windows.RtlGetVersion()

	mergedPath := basePath + ".merged"
	mergedPathPtr, err := windows.UTF16PtrFromString(mergedPath)
	if err != nil {
		return err
	}

	ret, _, _ = orSaveHive.Call(
		uintptr(merged),
		uintptr(unsafe.Pointer(mergedPathPtr)),
		uintptr(version.MajorVersion),
		uintptr(version.MinorVersion),
	)
	if ret != 0 {
		return fmt.Errorf("ORSaveHive: %w", windows.Errno(ret))
	}

	// the streamed base hive may carry a read-only attribute that would
	// prevent replacing it
	err = os.Chmod(basePath, 0644)
	if err != nil {
		return fmt.Errorf("clear base hive attributes: %w", err)
	}

	err = os.Rename(mergedPath, basePath)
	if err != nil {
		return fmt.Errorf("replace base hive: %w", err)
	}

	return nil
}

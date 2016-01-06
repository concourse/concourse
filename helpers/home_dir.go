package helpers

import (
	"io/ioutil"
	"os"
	"runtime"
)

func HomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func CreateTempHomeDir() error {
	tmpDir, err := ioutil.TempDir("", "fly-test")
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		os.Setenv("USERPROFILE", tmpDir)
	} else {
		os.Setenv("HOME", tmpDir)
	}

	return nil
}

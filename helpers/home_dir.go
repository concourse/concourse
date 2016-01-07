package helpers

import (
	"io/ioutil"
	"os"
	"runtime"
)

func CreateTempHomeDir() (string, error) {
	tmpDir, err := ioutil.TempDir("", "fly-test")
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		os.Setenv("USERPROFILE", tmpDir)
	} else {
		os.Setenv("HOME", tmpDir)
	}

	return tmpDir, nil
}

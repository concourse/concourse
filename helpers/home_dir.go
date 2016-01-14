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

	SetHomeDir(tmpDir)

	return tmpDir, nil
}

func SetHomeDir(dir string) {
	if runtime.GOOS == "windows" {
		os.Setenv("USERPROFILE", dir)
	} else {
		os.Setenv("HOME", dir)
	}
}

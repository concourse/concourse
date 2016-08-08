package helpers

import (
	"time"

	"github.com/concourse/go-concourse/concourse"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type Passed struct {
	FlyBin  string
	HomeDir string
	AtcURL  string
}

func FirstNodeFlySetup(atcURL, targetedConcourse string) ([]byte, error) {
	flyBin, err := gexec.Build("github.com/concourse/fly", "-race")
	if err != nil {
		return nil, err
	}

	tmpHome, err := CreateTempHomeDir()
	if err != nil {
		return nil, err
	}

	err = FlyLogin(atcURL, targetedConcourse, flyBin)
	if err != nil {
		return nil, err
	}

	data, err := EncodeStruct(Passed{
		FlyBin:  flyBin,
		HomeDir: tmpHome,
		AtcURL:  atcURL,
	})
	if err != nil {
		return nil, err
	}

	return data, nil
}

func AllNodeFlySetup(data []byte) (string, string, error) {
	var pass Passed
	err := DecodeStruct(data, &pass)
	if err != nil {
		return "", "", err
	}

	SetHomeDir(pass.HomeDir)

	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	return pass.FlyBin, pass.HomeDir, err
}

func FirstNodeClientSetup(atcURL string) ([]byte, error) {
	data, err := EncodeStruct(Passed{
		AtcURL: atcURL,
	})
	if err != nil {
		return nil, err
	}

	return data, nil
}

func AllNodeClientSetup(data []byte) (concourse.Client, error) {
	var pass Passed
	err := DecodeStruct(data, &pass)
	if err != nil {
		return nil, err
	}

	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(5 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	return ConcourseClient(pass.AtcURL), nil
}

package helpers

import (
	"errors"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func FlyLogin(atcURL, concourseAlias, flyBinaryPath string) error {
	dev, basicAuth, _, err := getAuthMethods(atcURL)
	if err != nil {
		return err
	}

	if dev {
		return nil
	} else if basicAuth != nil {
		return flyLoginWithBasicAuth(basicAuth.Username, basicAuth.Password, atcURL, concourseAlias, flyBinaryPath)
	}

	return errors.New("Unable to determine authentication")
}

func flyLoginWithBasicAuth(username, password, atcURL, concourseAlias, flyBinaryPath string) error {
	loginCmd := exec.Command(
		flyBinaryPath,
		"login",
		"-c", atcURL,
		"-t", concourseAlias,
		"-u", username,
		"-p", password,
	)

	loginProcess, err := gexec.Start(loginCmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}

	Eventually(loginProcess, time.Minute).Should(gbytes.Say("token saved"))
	Eventually(loginProcess, time.Minute).Should(gexec.Exit(0))

	return nil
}

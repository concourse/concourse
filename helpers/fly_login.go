package helpers

import (
	"errors"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func FlyLogin(atcURL, concourseAlias, flyBinaryPath string) error {
	dev, basicAuth, _, err := getAuthMethods(atcURL)
	if err != nil {
		return err
	}

	if dev {
		return flyLogin(flyBinaryPath, []string{
			"-c", atcURL,
			"-t", concourseAlias,
		})
	} else if basicAuth != nil {
		return flyLogin(flyBinaryPath, []string{
			"-c", atcURL,
			"-t", concourseAlias,
			"-u", basicAuth.Username,
			"-p", basicAuth.Password,
		})
	}

	return errors.New("Unable to determine authentication")
}

func flyLogin(flyBinaryPath string, loginArgs []string) error {
	args := append([]string{"login"}, loginArgs...)
	loginCmd := exec.Command(flyBinaryPath, args...)

	loginProcess, err := gexec.Start(loginCmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}

	Eventually(loginProcess, time.Minute).Should(gexec.Exit(0))

	return nil
}

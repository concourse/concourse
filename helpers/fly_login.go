package helpers

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func FlyLogin(atcURL, concourseAlias, flyBinaryPath string) error {
	return flyLogin(flyBinaryPath, []string{
		"-c", atcURL,
		"-t", concourseAlias,
	})
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

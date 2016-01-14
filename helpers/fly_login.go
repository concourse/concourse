package helpers

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func FlyLogin(atcURL, concourseAlias, flyBinaryPath string, loginInfo LoginInformation) error {
	switch {
	case loginInfo.NoAuth:
		return flyLogin(flyBinaryPath, []string{
			"-c", atcURL,
			"-t", concourseAlias,
		})
	case loginInfo.BasicAuthCreds.Username != "":
		return flyLogin(flyBinaryPath, []string{
			"-c", atcURL,
			"-t", concourseAlias,
			"-u", loginInfo.BasicAuthCreds.Username,
			"-p", loginInfo.BasicAuthCreds.Password,
		})
	case loginInfo.OauthToken != "":
		return flyLoginOauth(flyBinaryPath, atcURL, loginInfo.OauthToken, []string{
			"-c", atcURL,
			"-t", concourseAlias,
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

func flyLoginOauth(flyBinaryPath string, atcURL string, oauthToken string, loginArgs []string) error {
	args := append([]string{"login"}, loginArgs...)
	loginCmd := exec.Command(flyBinaryPath, args...)

	stdin, err := loginCmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()

	loginProcess, err := gexec.Start(loginCmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}

	Eventually(loginProcess.Out).Should(gbytes.Say("enter token"))
	fmt.Fprint(stdin, fmt.Sprintf("%s\n", oauthToken))

	Eventually(loginProcess, time.Minute).Should(gexec.Exit(0))

	return nil
}

package bosh

import (
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func DeleteDeployment(deploymentName string) {
	deleteDeployment := exec.Command("bosh", "-n", "delete", "deployment", deploymentName)

	session, err := gexec.Start(deleteDeployment, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(session, 5*time.Minute).Should(gexec.Exit())

	if strings.Contains(string(session.Err.Contents()), "doesn't exist") {
		return
	}

	Ω(session.ExitCode()).Should(Equal(0))
}

package bosh

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func DeleteDeployment(deploymentName string) {
	deleteDeployment := exec.Command("bosh", "-n", "delete", "deployment", deploymentName, "--force")

	session, err := gexec.Start(
		deleteDeployment,
		gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[bosh]", "black+bright")),
			GinkgoWriter,
		),
		gexec.NewPrefixedWriter(
			fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[bosh]", "black+bright")),
			GinkgoWriter,
		),
	)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(session, 5*time.Minute).Should(gexec.Exit())

	if strings.Contains(string(session.Err.Contents()), "doesn't exist") {
		return
	}

	Ω(session.ExitCode()).Should(Equal(0))
}

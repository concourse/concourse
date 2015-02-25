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

func DirectorUUID() string {
	directorUUID := exec.Command("bosh", "status", "--uuid")

	session, err := gexec.Start(
		directorUUID,
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

	return strings.TrimSpace(string(session.Out.Contents()))
}

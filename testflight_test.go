package testflight_test

import (
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Running a build with Smith", func() {
	It("works", func() {
		smith := exec.Command(builtComponents["smith"])
		smith.Dir = filepath.Join(fixturesDir, "trivial-build")

		session, err := gexec.Start(smith, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session, 10*time.Minute).Should(gexec.Exit(0))

		Ω(session).Should(gbytes.Say("some output"))
		Ω(session).Should(gbytes.Say("FOO is 1"))
	})
})

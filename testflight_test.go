package flight_test_test

import (
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Running a build with Led", func() {
	It("works", func() {
		smith := exec.Command(builtComponents["smith"])
		smith.Dir = filepath.Join(fixturesDir, "trivial-build")

		session, err := gexec.Start(smith, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session, 3000000).Should(gexec.Exit())

		Ω(session.ExitCode()).Should(Equal(0))
	})
})

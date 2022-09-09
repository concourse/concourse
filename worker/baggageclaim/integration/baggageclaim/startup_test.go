package integration_test

import (
	"os/exec"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Startup", func() {
	var (
		process *gexec.Session
	)

	AfterEach(func() {
		process.Kill().Wait(1 * time.Second)
	})

	It("exits with an error if --volumes is not specified", func() {
		port := 7788 + GinkgoParallelNode()

		command := exec.Command(
			baggageClaimPath,
			"--bind-port", strconv.Itoa(port),
		)

		var err error
		process, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(process.Err).Should(gbytes.Say("the required flag `(--|/)volumes' was not specified"))
		Eventually(process).Should(gexec.Exit(1))
	})
})

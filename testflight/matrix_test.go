package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with a complicated build plan", func() {
	var initialVersion string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		initialVersion = u.String()

		setAndUnpausePipeline(
			"fixtures/matrix.yml",
			"-v", "initial_version="+initialVersion,
		)
	})

	It("executes the build plan correctly", func() {
		watch := spawnFly("trigger-job", "-j", inPipeline("fancy-build-matrix"), "-w")
		<-watch.Exited
		Expect(watch.ExitCode()).To(Equal(1)) // expect failure
		Expect(watch).To(gbytes.Say("passing-unit-1/file passing-unit-2/file " + initialVersion))
		Expect(watch).To(gbytes.Say("failed aggregate " + initialVersion))
	})
})

package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Checking a resource", func() {
	var hash string
	var checkFailure string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		hash = u.String()
	})

	JustBeforeEach(func() {
		setAndUnpausePipeline(
			"fixtures/resource-check.yml",
			"-v", "unique_version="+hash,
			"-v", "check_failure="+checkFailure,
		)
	})

	It("saves the versions", func() {
		check := fly("check-resource", "-r", inPipeline("my-resource"))
		Expect(check).To(gbytes.Say("my-resource"))
		Expect(check).To(gbytes.Say("succeeded"))

		versions := fly("resource-versions", "-r", inPipeline("my-resource"))
		Expect(versions).To(gbytes.Say(hash))
	})

	Context("when checking fails", func() {
		BeforeEach(func() {
			checkFailure = "super broken"
		})

		It("shows the check status failed", func() {
			check := spawnFly("check-resource", "-r", inPipeline("my-resource"))
			<-check.Exited
			Expect(check).To(gexec.Exit(1))
			Expect(check).To(gbytes.Say("super broken"))
			Expect(check).To(gbytes.Say("failed"))

			resources := fly("resources", "-p", pipelineName)
			Expect(resources).To(gbytes.Say("failed"))
		})
	})
})

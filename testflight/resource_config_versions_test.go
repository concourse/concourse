package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource config versions", func() {
	BeforeEach(func() {
		hash, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/resource-type-versions.yml",
			"-v", "hash="+hash.String(),
		)
	})

	// This test is for a case where the build inputs and outputs will not be
	// invalidated if the resource config id field on the resource gets updated
	// due to a new version of the custom resource type that it is using.
	Describe("build inputs and outputs are not affected by a change in resource config id", func() {
		It("will run both jobs only once even with a new custom resource type version", func() {
			By("waiting for a new build when the pipeline is created")
			fly("trigger-job", "-j", inPipeline("initial-job"), "-w")

			By("checking the a new version of the custom resource type")
			u, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			newVersion := u.String()

			fly("check-resource-type", "-r", inPipeline("custom-resource-type"), "-f", "version:"+newVersion)

			By("triggering a job using the custom type")
			fly("trigger-job", "-j", inPipeline("passed-job"), "-w")

			By("using the version  of 'some-resource' consumed upstream")
			Expect(flyTable("builds", "-j", inPipeline("initial-job"))).To(HaveLen(1))
		})
	})
})

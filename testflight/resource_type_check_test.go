package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Resource-types checks", func() {
	BeforeEach(func() {
		hash, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/resource-types.yml",
			"-v", "hash="+hash.String(),
		)

		checkS := fly("check-resource", "-r", inPipeline("my-resource-image"))
		Eventually(checkS).Should(gbytes.Say("succeeded"))
	})

	It("can check the resource-type", func() {
		checkS := fly("check-resource-type", "-r", inPipeline("custom-resource-type"))
		Eventually(checkS).Should(gbytes.Say("succeeded"))
	})

	Context("when there is a new version", func() {
		var newVersion string

		BeforeEach(func() {
			u, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())

			newVersion = u.String()

			fly("check-resource-type", "-r", inPipeline("custom-resource-type"), "-f", "version:"+newVersion)
		})

		It("uses the updated resource type", func() {
			watch := fly("trigger-job", "-j", inPipeline("resource-imager"), "-w")
			Expect(watch).To(gbytes.Say("MIRRORED_VERSION=" + newVersion))
		})
	})

	Context("when the resource-type check fails", func() {
		It("fails", func() {
			watch := spawnFly("check-resource-type", "-r", inPipeline("failing-custom-resource-type"))
			Eventually(watch.Out).Should(gbytes.Say("failed"))
			Eventually(watch).Should(gexec.Exit(1))
		})
	})
})

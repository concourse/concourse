package pipelines_test

import (
	"os/exec"
	"time"

	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that produces outputs", func() {
	Context("with outputs and single worker", func() {
		BeforeEach(func() {
			configurePipeline(
				"-c", "fixtures/task-outputs.yml",
			)
		})

		It("propagates the outputs from one task to another", func() {
			watch := triggerJob("some-job")
			<-watch.Exited
			Expect(watch).To(gbytes.Say("initializing"))
			Expect(watch).To(gexec.Exit(0))

			Expect(watch.Out.Contents()).To(ContainSubstring("./output-1/file-1"))
			Expect(watch.Out.Contents()).To(ContainSubstring("./output-2/file-2"))
		})
	})

	Context("hijacking with outputs and multiple workers", func() {
		BeforeEach(func() {
			configurePipeline(
				"-c", "fixtures/task-outputs-tagged.yml",
			)

			if !hasTaggedWorkers() {
				Skip("this only runs when a worker with the 'tagged' tag is available")
			}
		})

		It("can hijack to task which produces outputs (see #123243131)", func() {
			watch := triggerJob("some-job")
			<-watch.Exited
			Expect(watch).To(gexec.Exit(0))

			hijack := exec.Command(flyBin, "-t", targetedConcourse, "hijack",
				"-j", pipelineName+"/some-job",
				"-s", "output-producer",
				"--", "sh", "-c",
				"echo ok",
			)

			hijackS := helpers.StartFly(hijack)
			Eventually(hijackS).Should(gexec.Exit(0))
			Eventually(hijackS, 30*time.Second).Should(gbytes.Say("ok"))
		})
	})
})

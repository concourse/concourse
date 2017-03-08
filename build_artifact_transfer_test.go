package topgun_test

import (
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Passing artifacts between build steps", func() {
	BeforeEach(func() {
		Deploy("deployments/two-workers-different-types.yml")
	})

	It("transfers bits between workers when the resource type is not supported", func() {
		By("setting pipeline that creates containers for check, get, task, put")
		fly("set-pipeline", "-n", "-c", "pipelines/build-artifact-transfer.yml", "-p", "build-artifacts")

		By("unpausing the pipeline")
		fly("unpause-pipeline", "-p", "build-artifacts")

		By("triggering job")
		sess := spawnFly("trigger-job", "-w", "-j", "build-artifacts/transfer-time")
		<-sess.Exited
		Expect(sess).To(gbytes.Say("./special-time/input"))
		Expect(sess.ExitCode()).To(Equal(0))
	})
})

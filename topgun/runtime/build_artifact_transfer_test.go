package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	_ "github.com/jackc/pgx/v5/stdlib"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Passing artifacts between build steps", func() {
	BeforeEach(func() {
		Deploy(
			"deployments/concourse.yml",
			"-o", "operations/add-other-worker.yml",
			"-o", "operations/distinct-worker-tags.yml",
		)
	})

	It("transfers bits between workers", func() {
		By("setting pipeline that creates containers for check, get, task, put")
		Fly.Run("set-pipeline", "-n", "-c", "pipelines/build-artifact-transfer.yml", "-p", "build-artifacts")

		By("unpausing the pipeline")
		Fly.Run("unpause-pipeline", "-p", "build-artifacts")

		By("triggering job")
		sess := Fly.Start("trigger-job", "-w", "-j", "build-artifacts/transfer-time")
		Eventually(sess).Should(gexec.Exit(0))
		Expect(sess).To(gbytes.Say("./something/version"))
	})
})

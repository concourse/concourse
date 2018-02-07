package topgun_test

import (
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Resource Certs", func() {
	BeforeEach(func() {
		Deploy("deployments/concourse-different-workers.yml",
			"-o", "operations/other-worker-no-certs.yml",
		)
		waitForWorkersToBeRunning()
	})

	Context("with a certs path configured on the resource", func() {
		BeforeEach(func() {
			By("setting a pipeline that has a tagged resource")
			fly("set-pipeline", "-n", "-c", "pipelines/certs-tagged-resources.yml", "-p", "resources")

			By("unpausing the pipeline pipeline")
			fly("unpause-pipeline", "-p", "resources")
		})

		It("bind mounts the certs volume if the worker has one", func() {
			By("running the checks")
			fly("check-resource", "-r", "resources/no-certs")
			fly("check-resource", "-r", "resources/certs")

			hijackSession := spawnFly("hijack", "-c", "resources/no-certs", "--", "ls", "/etc/ssl/certs")
			<-hijackSession.Exited

			certsContent := string(hijackSession.Out.Contents())
			Expect(certsContent).To(HaveLen(0))

			hijackSession = spawnFly("hijack", "-c", "resources/certs", "--", "ls", "/etc/ssl/certs")
			<-hijackSession.Exited

			certsContent = string(hijackSession.Out.Contents())
			Expect(certsContent).ToNot(HaveLen(0))
		})

		It("bind mounts the certs volume to resource get containers", func() {
			trigger := spawnFly("trigger-job", "-j", "resources/use-em")
			<-trigger.Exited

			Eventually(func() string {
				builds := flyTable("builds", "-j", "resources/use-em")
				return builds[0]["status"]
			}).Should(Equal("failed"))

			hijackSession := spawnFly("hijack", "-j", "resources/use-em", "-s", "certs", "--", "ls", "/etc/ssl/certs")
			<-hijackSession.Exited
			certsContent := string(hijackSession.Out.Contents())
			Expect(certsContent).ToNot(HaveLen(0))
		})

		It("bind mounts the certs volume to resource put containers", func() {
			trigger := spawnFly("trigger-job", "-j", "resources/use-em")
			<-trigger.Exited

			Eventually(func() string {
				builds := flyTable("builds", "-j", "resources/use-em")
				return builds[0]["status"]
			}).Should(Equal("failed"))

			hijackSession := spawnFly("hijack", "-j", "resources/use-em", "-s", "put-certs", "--", "ls", "/etc/ssl/certs")
			Eventually(hijackSession).Should(gexec.Exit(0))

			certsContent := string(hijackSession.Out.Contents())
			Expect(certsContent).ToNot(HaveLen(0))
		})

	})
})

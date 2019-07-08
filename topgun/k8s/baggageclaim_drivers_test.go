package k8s_test

import (
	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("baggageclaim drivers", func() {

	AfterEach(func() {
		cleanup(releaseName, namespace, nil)
	})

	onPks(func() {
		works("btrfs")
		works("overlay")
		works("naive")
	})

	onGke(func() {

		const (
			COS    = "--set=worker.nodeSelector.nodeImage=cos"
			UBUNTU = "--set=worker.nodeSelector.nodeImage=ubuntu"
		)

		Context("cos image", func() {
			fails("btrfs", COS)
			works("overlay", COS)
			works("naive", COS)
		})

		Context("ubuntu image", func() {
			works("btrfs", UBUNTU)
			works("overlay", UBUNTU)
			works("naive", UBUNTU)
		})

	})
})

func onPks(f func()) {
	Context("PKS", func() {

		BeforeEach(func() {
			if Environment.K8sEngine != "PKS" {
				Skip("not running on PKS")
			}
		})

		f()
	})
}

func onGke(f func()) {
	Context("GKE", func() {

		BeforeEach(func() {
			if Environment.K8sEngine != "GKE" {
				Skip("not running on GKE")
			}
		})

		f()
	})
}

func works(driver string, selectorFlags ...string) {
	Context(driver, func() {
		It("works", func() {
			deployWithSelectors(driver, selectorFlags...)
			waitAllPodsInNamespaceToBeReady(namespace)

			By("Creating the web proxy")
			_, atcEndpoint := startPortForwarding(namespace, "service/"+releaseName+"-web", "8080")

			By("Logging in")
			fly.Login("test", "test", atcEndpoint)

			By("Setting and triggering a dumb pipeline")
			fly.Run("set-pipeline", "-n", "-c", "../pipelines/get-task.yml", "-p", "some-pipeline")
			fly.Run("unpause-pipeline", "-p", "some-pipeline")
			fly.Run("trigger-job", "-w", "-j", "some-pipeline/simple-job")
		})
	})
}

func fails(driver string, selectorFlags ...string) {
	Context(driver, func() {
		It("fails", func() {
			deployWithSelectors(driver, selectorFlags...)

			Eventually(func() []byte {
				workerLogsSession := Start(nil, "kubectl", "logs",
					"--namespace="+namespace, "-lapp="+namespace+"-worker")
				<-workerLogsSession.Exited

				return workerLogsSession.Out.Contents()

			}).Should(ContainSubstring("failed-to-set-up-driver"))

		})
	})
}

func deployWithSelectors(driver string, selectorFlags ...string) {
	setReleaseNameAndNamespace("bd-" + driver)

	helmDeployTestFlags := []string{
		"--set=concourse.web.kubernetes.enabled=false",
		"--set=concourse.worker.baggageclaim.driver=" + driver,
		"--set=worker.replicas=1",
	}

	deployConcourseChart(releaseName, append(helmDeployTestFlags, selectorFlags...)...)
}

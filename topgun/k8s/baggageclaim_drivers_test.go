package k8s_test

import (
	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("baggageclaim drivers", func() {

	AfterEach(func() {
		cleanup(releaseName, namespace)
	})

	onPks(func() {
		baggageclaimWorks("btrfs")
		baggageclaimWorks("overlay")
		baggageclaimWorks("naive")
	})

	onGke(func() {

		const (
			COS    = "--set=worker.nodeSelector.nodeImage=cos"
			UBUNTU = "--set=worker.nodeSelector.nodeImage=ubuntu"
		)

		Context("cos image", func() {
			baggageclaimFails("btrfs", COS)
			baggageclaimWorks("overlay", COS)
			baggageclaimWorks("naive", COS)
		})

		Context("ubuntu image", func() {
			baggageclaimWorks("btrfs", UBUNTU)
			baggageclaimWorks("overlay", UBUNTU)
			baggageclaimWorks("naive", UBUNTU)
		})

	})
})

func baggageclaimWorks(driver string, selectorFlags ...string) {
	Context(driver, func() {
		It("works", func() {
			setReleaseNameAndNamespace("bd-" + driver)
			deployWithDriverAndSelectors(driver, selectorFlags...)

			atc := waitAndLogin(namespace, releaseName+"-web")
			defer atc.Close()

			By("Setting and triggering a dumb pipeline")
			fly.Run("set-pipeline", "-n", "-c", "../pipelines/get-task.yml", "-p", "some-pipeline")
			fly.Run("unpause-pipeline", "-p", "some-pipeline")
			fly.Run("trigger-job", "-w", "-j", "some-pipeline/simple-job")
		})
	})
}

func baggageclaimFails(driver string, selectorFlags ...string) {
	Context(driver, func() {
		It("fails", func() {
			setReleaseNameAndNamespace("bd-" + driver)
			deployWithDriverAndSelectors(driver, selectorFlags...)

			Eventually(func() []byte {
				workerLogsSession := Start(nil, "kubectl", "logs",
					"--namespace="+namespace, "-lapp="+namespace+"-worker")
				<-workerLogsSession.Exited

				return workerLogsSession.Out.Contents()

			}).Should(ContainSubstring("failed-to-set-up-driver"))
		})
	})
}

func deployWithDriverAndSelectors(driver string, selectorFlags ...string) {
	helmDeployTestFlags := []string{
		"--set=concourse.web.kubernetes.enabled=false",
		"--set=concourse.worker.baggageclaim.driver=" + driver,
		"--set=worker.replicas=1",
	}

	deployConcourseChart(releaseName, append(helmDeployTestFlags, selectorFlags...)...)
}

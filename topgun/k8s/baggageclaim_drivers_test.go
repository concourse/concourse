package k8s_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		Context("with a real btrfs partition", func() {
			It("successfully recreates the worker", func() {
				By("deploying concourse with ONLY one worker and having the worker pod use the gcloud disk and format it with btrfs")

				setReleaseNameAndNamespace("real-btrfs-disk")

				deployWithDriverAndSelectors("btrfs", UBUNTU,
					"--set=persistence.enabled=false",
					"--set=worker.additionalVolumes[0].name=concourse-work-dir",
					"--set=worker.additionalVolumes[0].gcePersistentDisk.pdName=disk-topgun-k8s-btrfs-test",
					"--set=worker.additionalVolumes[0].gcePersistentDisk.fsType=btrfs",
				)

				atc := waitAndLogin(namespace, releaseName+"-web")
				defer atc.Close()

				By("Setting and triggering a pipeline that always fails which creates volumes on the persistent disk")
				fly.Run("set-pipeline", "-n", "-c", "pipelines/pipeline-that-fails.yml", "-p", "failing-pipeline")
				fly.Run("unpause-pipeline", "-p", "failing-pipeline")
				sessionTriggerJob := fly.Start("trigger-job", "-w", "-j", "failing-pipeline/simple-job")
				<-sessionTriggerJob.Exited

				By("deleting the worker pod which triggers the initContainer script")
				deletePods(releaseName, fmt.Sprintf("--selector=app=%s-worker", releaseName))

				By("all pods should be running")
				waitAllPodsInNamespaceToBeReady(namespace)
			})
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
			fly.Run("set-pipeline", "-n", "-c", "pipelines/get-task.yml", "-p", "some-pipeline")
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
				var logs []byte
				pods := getPods(namespace, metav1.ListOptions{LabelSelector: "app=" + namespace + "-worker"})
				for _, p := range pods {
					contents, _ := kubeClient.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{}).Do().Raw()
					logs = append(logs, contents...)
				}

				return logs

			}, 2*time.Minute, 1*time.Second).Should(ContainSubstring("failed-to-set-up-driver"))
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

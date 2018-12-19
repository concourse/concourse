package k8s_test

import (
	"fmt"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Baggageclaim Drivers", func() {
	var (
		proxySession *gexec.Session
		releaseName  string
		namespace    string
		atcEndpoint  string
		driver string
	)

	JustBeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-bd-%s-%d-%d", driver, GinkgoRandomSeed(), GinkgoParallelNode())
		namespace = releaseName

		deployConcourseChart(releaseName,
			"--set=worker.replicas=1",
			"--set=concourse.web.kubernetes.enabled=false",
			"--set=concourse.worker.baggageclaim.driver=" + driver)

		waitAllPodsInNamespaceToBeReady(namespace)

		By("Creating the web proxy")
		proxySession, atcEndpoint = startPortForwarding(namespace, releaseName+"-web", "8080")

		By("Logging in")
		fly.Login("test", "test", atcEndpoint)

		By("waiting for a running worker")
		Eventually(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 2*time.Minute, 10*time.Second).
			ShouldNot(HaveLen(0))
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
		Wait(proxySession.Interrupt())
	})

	// TODO - Investigate how to make it work properly on GKE
	XContext("btrfs", func () {
		BeforeEach(func () {
			driver = "btrfs"
		})

		It("works", func() {
			fly.Run("set-pipeline", "-n", "-c", "../pipelines/get-task.yml", "-p", "pipeline")
			fly.Run("trigger-job", "-j", "pipeline/simple-job")
		})
	})

	Context("overlay", func () {
		BeforeEach(func () {
			driver = "overlay"
		})

		It("works", func() {
			fly.Run("set-pipeline", "-n", "-c", "../pipelines/get-task.yml", "-p", "pipeline")
			fly.Run("trigger-job", "-j", "pipeline/simple-job")
		})
	})

	Context("naive", func () {
		BeforeEach(func () {
			driver = "naive"
		})

		It("works", func() {
			fly.Run("set-pipeline", "-n", "-c", "../pipelines/get-task.yml", "-p", "pipeline")
			fly.Run("trigger-job", "-j", "pipeline/simple-job")
		})
	})

})


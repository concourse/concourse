package k8s_test

import (
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker lifecycle", func() {

	var (
		proxySession *gexec.Session
		atcEndpoint  string
		gracePeriod  string
	)

	JustBeforeEach(func() {
		setReleaseNameAndNamespace("wl")

		deployConcourseChart(releaseName,
			`--set=worker.replicas=1`,
			`--set=persistence.enabled=false`,
			`--set=worker.terminationGracePeriodSeconds=`+gracePeriod,
		)

		waitAllPodsInNamespaceToBeReady(namespace)

		By("Creating the web proxy")
		proxySession, atcEndpoint = startPortForwarding(
			namespace, "service/"+releaseName+"-web", "8080",
		)

		By("Logging in")
		fly.Login("test", "test", atcEndpoint)

		By("waiting for a running worker")
		Eventually(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 2*time.Minute, 10*time.Second).
			ShouldNot(HaveLen(0))

		fly.Run("set-pipeline", "-n",
			"-c", "../pipelines/task-waiting.yml",
			"-p", "some-pipeline",
		)

		fly.Run("unpause-pipeline", "-p", "some-pipeline")
		fly.Run("trigger-job", "-j", "some-pipeline/simple-job")

		By("waiting container to be created")
		Eventually(func() bool {
			containers := fly.GetContainers()

			for _, container := range containers {
				if container.Type == "task" && container.State == "created" {
					return true
				}
			}

			return false
		}, 2*time.Minute, 10*time.Second).
			Should(BeTrue())

		Run(nil, "kubectl", "scale", "--namespace", namespace,
			"statefulset", releaseName+"-worker", "--replicas=0",
		)
	})

	AfterEach(func() {
		cleanup(releaseName, namespace, proxySession)
	})

	Context("terminating the worker", func() {

		Context("gracefully", func() {
			BeforeEach(func() {
				gracePeriod = "600"
			})

			It("finishes tasks gracefully with termination", func() {
				By("seeing that the worker state is retiring")
				Eventually(func() string {
					workers := fly.GetWorkers()
					Expect(workers).To(HaveLen(1))
					return workers[0].State
				}, 10*time.Second, 2*time.Second).
					Should(Equal("retiring"))

				By("letting the task finish")
				fly.Run("hijack", "--verbose", "-j", "some-pipeline/simple-job",
					"--", "/bin/sh", "-ce",
					`touch /tmp/stop-waiting`,
				)

				By("seeing that there are no workers")
				Eventually(func() []Worker {
					return getRunningWorkers(fly.GetWorkers())
				}, 1*time.Minute, 1*time.Second).
					Should(HaveLen(0))

				By("seeing that the build succeeded")
				Eventually(func() *gbytes.Buffer {
					startSession := fly.Start("builds", "-j", "some-pipeline/simple-job")
					<-startSession.Exited
					return startSession.Out
				}, 1*time.Minute, 5*time.Second).
					Should(gbytes.Say("succeeded"))
			})
		})

		Context("ungracefully", func() {
			BeforeEach(func() {
				gracePeriod = "1"
			})

			It("interrupts the task execution", func() {
				By("seeing that there are no workers")

				Eventually(func() []Worker {
					return getRunningWorkers(fly.GetWorkers())
				}, 1*time.Minute, 1*time.Second).
					Should(HaveLen(0))

				By("seeing that the worker disappeared")
				startSession := fly.Start("watch", "-j", "some-pipeline/simple-job")
				<-startSession.Exited
				Expect(startSession.Out).To(gbytes.Say("disappeared"))
			})
		})
	})
})

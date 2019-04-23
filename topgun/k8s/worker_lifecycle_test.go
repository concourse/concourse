package k8s_test

import (
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker lifecycle", func() {
	var (
		proxySession        *gexec.Session
		atcEndpoint         string
		helmDeployTestFlags []string
	)

	type Case struct {
		TerminationGracePeriod    string
		PipelineYamlFile          string
		WorkerDisappearingTimeout time.Duration
		RerunJob                  bool
	}

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
		Wait(proxySession.Interrupt())
	})

	DescribeTable("retiring a worker",
		func(c Case) {
			setReleaseNameAndNamespace("wl")

			helmDeployTestFlags = []string{
				`--set=worker.replicas=1`,
				`--set=worker.terminationGracePeriodSeconds=` + c.TerminationGracePeriod,
			}
			deployConcourseChart(releaseName, helmDeployTestFlags...)

			waitAllPodsInNamespaceToBeReady(namespace)

			By("Creating the web proxy")
			proxySession, atcEndpoint = startPortForwarding(namespace, "service/"+releaseName+"-web", "8080")

			By("Logging in")
			fly.Login("test", "test", atcEndpoint)

			Eventually(func() []Worker {
				return getRunningWorkers(fly.GetWorkers())
			}, 2*time.Minute, 10*time.Second).
				ShouldNot(HaveLen(0))

			time.Sleep(2 * time.Second)

			fly.Run("set-pipeline", "-n", "-c", "../pipelines/"+c.PipelineYamlFile, "-p", "some-pipeline")
			fly.Run("unpause-pipeline", "-p", "some-pipeline")
			fly.Run("trigger-job", "-j", "some-pipeline/simple-job")

			Eventually(func() *gbytes.Buffer {
				startSession := fly.Start("builds", "-j", "some-pipeline/simple-job")
				<-startSession.Exited
				return startSession.Out
			}, 1*time.Minute, 1*time.Second).Should(gbytes.Say(".*started.*"))

			time.Sleep(2 * time.Second)

			By("deleting the worker pod")
			Run(nil, "kubectl", "delete", "pod", "--namespace", namespace, releaseName+"-worker-0", "--wait=false")

			By("seeing that the worker state is retiring")
			Eventually(func() string {
				workers := fly.GetWorkers()
				Expect(workers).To(HaveLen(1))
				return workers[0].State
			}, 10*time.Second, 2*time.Second).
				Should(Equal("retiring"))

			By("seeing that there are no workers")
			Eventually(func() []Worker {
				return getRunningWorkers(fly.GetWorkers())
			}, c.WorkerDisappearingTimeout, 1*time.Second).
				Should(HaveLen(0))

			By("seeing that workers have been revived")
			Eventually(func() []Worker {
				return getRunningWorkers(fly.GetWorkers())
			}, 2*time.Minute, 10*time.Second).
				ShouldNot(HaveLen(0))

			if c.RerunJob {
				By("making sure the first build has succeeded")
				startSession := fly.Start("builds", "-j", "some-pipeline/simple-job")
				<-startSession.Exited
				Expect(startSession.Out).To(gbytes.Say("succeeded"))

				By("running the same job again and watching it to completion")
				fly.Run("trigger-job", "-j", "some-pipeline/simple-job", "-w")
			}
		},
		Entry("gracefully", Case{
			TerminationGracePeriod:    "600",
			PipelineYamlFile:          "simple-pipeline.yml",
			WorkerDisappearingTimeout: 1 * time.Minute,
			RerunJob:                  true,
		}),
		Entry("ungracefully", Case{
			TerminationGracePeriod:    "30",
			PipelineYamlFile:          "task-waiting.yml",
			WorkerDisappearingTimeout: 40 * time.Second,
			RerunJob:                  false,
		}),
	)
})

package k8s_test

import (
	"bufio"
	"bytes"
	"fmt"
	"path"
	"time"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func HelmDeploy(releaseName string) {
	helmArgs := []string{
		"upgrade",
		"-f",
		path.Join(Environment.ChartDir, "values.yaml"),
		"--install",
		"--force",
		"--set=concourse.web.kubernetes.keepNamespaces=false",
		// TODO: https://github.com/concourse/concourse/issues/2827
		"--set=concourse.web.gc.interval=300ms",
		"--set=concourse.web.tsa.heartbeatInterval=300ms",
		"--set=concourse.worker.ephemeral=true",
		"--set=worker.replicas=1",
		"--set=concourse.worker.baggageclaim.driver=btrfs",
		"--set=image=" + Environment.ConcourseImageName,
		"--set=imageDigest=" + Environment.ConcourseImageDigest,
		"--set=imageTag=" + Environment.ConcourseImageTag,
		releaseName,
		"--wait",
		Environment.ChartDir,
	}

	Wait(Start(nil, "helm", helmArgs...))
}

func HelmDestroy(releaseName string) {
	helmArgs := []string{
		"delete",
		releaseName,
	}

	Wait(Start(nil, "helm", helmArgs...))
}

func getPods(releaseName string, flags ...string) []string {
	var (
		podNames = []string{}
		args     = append([]string{"get", "pods",
			"--selector=release=" + releaseName,
			"--output=name",
			"--sort-by={.metadata.name}",
			"--no-headers"}, flags...)
		session = Start(nil, "kubectl", args...)
	)

	Wait(session)

	scanner := bufio.NewScanner(bytes.NewBuffer(session.Out.Contents()))
	for scanner.Scan() {
		podNames = append(podNames, scanner.Text())
	}

	return podNames
}

func deletePods(releaseName string, flags ...string) []string {
	var (
		podNames = []string{}
		args     = append([]string{"delete", "pod",
			"--selector=release=" + releaseName,
		}, flags...)
		session = Start(nil, "kubectl", args...)
	)

	Wait(session)

	scanner := bufio.NewScanner(bytes.NewBuffer(session.Out.Contents()))
	for scanner.Scan() {
		podNames = append(podNames, scanner.Text())
	}

	return podNames
}

func getRunningPods(releaseName string) []string {
	return getPods(releaseName, "--field-selector=status.phase=Running")
}

func StartWebProxy(releaseName string, port int) *gexec.Session {
	session := Start(nil, "kubectl", "port-forward", "service/"+releaseName+"-web", fmt.Sprintf("%d:8080", port))
	Eventually(session.Out).Should(gbytes.Say("Forwarding"))
	return session
}

var _ = Describe("Ephemeral workers", func() {
	var (
		proxySession *gexec.Session
		releaseName  string
		atcEndpoint  string
	)

	BeforeEach(func() {
		port := 8080 + GinkgoParallelNode()
		releaseName = fmt.Sprintf("topgun-ephemeral-workers-%d", GinkgoParallelNode() )
		HelmDeploy(releaseName)

		Eventually(func() bool {
			expectedPods := getPods(releaseName)
			actualPods := getRunningPods(releaseName)

			return len(expectedPods) == len(actualPods)
		}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "expected all pods to be running")

		By("Creating the web proxy")
		proxySession = StartWebProxy(releaseName, port)
		atcEndpoint = fmt.Sprintf("http://127.0.0.1:%d", port)
	})

	AfterEach(func() {
		HelmDestroy(releaseName)
		Wait(proxySession.Interrupt())
	})

	It("Gets properly cleaned when getting removed and then put back on", func() {
		By("Logging in")
		fly.Login("test", "test", atcEndpoint)

		By("waiting for a running worker")
		Eventually(func() []Worker {
			return getRunningWorkers(fly.GetWorkers())
		}, 2*time.Minute, 10*time.Second).
			ShouldNot(HaveLen(0))

		deletePods(releaseName, fmt.Sprintf("--selector=app=%s-worker", releaseName))

		Eventually(func() (runningWorkers []Worker) {
			workers := fly.GetWorkers()
			for _, w := range workers {
				Expect(w.State).ToNot(Equal("stalled"), "the worker should never stall")
				if w.State == "running" {
					runningWorkers = append(runningWorkers, w)
				}
			}
			return
		}, 1*time.Minute, 1*time.Second).Should(HaveLen(0), "the running worker should go away")
	})
})

func getRunningWorkers(workers []Worker) (running []Worker) {
	for _, w := range workers {
		if w.State == "running" {
			running = append(running, w)
		}
	}
	return
}

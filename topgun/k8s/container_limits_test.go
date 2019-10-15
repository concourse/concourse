package k8s_test

import (
	"github.com/onsi/gomega/gexec"
	"time"

	"github.com/onsi/gomega/gbytes"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container Limits", func() {
	const (
		TaskCPULimit    = "--set=concourse.web.defaultTaskCpuLimit=512"
		TaskMemoryLimit = "--set=concourse.web.defaultTaskMemoryLimit=1GB"
		COS             = "--set=worker.nodeSelector.nodeImage=cos"
		UBUNTU          = "--set=worker.nodeSelector.nodeImage=ubuntu"
	)

	BeforeEach(func() {
		setReleaseNameAndNamespace("cl")
	})

	onPks(func() {
		containerLimitsWork(TaskCPULimit, TaskMemoryLimit)
	})

	onGke(func() {
		containerLimitsWork(COS, TaskCPULimit, TaskMemoryLimit)
		containerLimitsFail(UBUNTU, TaskCPULimit, TaskMemoryLimit)
	})

	AfterEach(func() {
		cleanup(releaseName, namespace, nil)
	})

})

func waitAndLogin() {
	waitAllPodsInNamespaceToBeReady(namespace)

	By("Creating the web proxy")
	_, atcEndpoint := startPortForwarding(namespace, "service/"+releaseName+"-web", "8080")

	By("Logging in")
	fly.Login("test", "test", atcEndpoint)

	Eventually(func() []Worker {
		return getRunningWorkers(fly.GetWorkers())
	}, 2*time.Minute, 10*time.Second).
		ShouldNot(HaveLen(0))

}

func deployWithSelectors(selectorFlags ...string) {
	helmDeployTestFlags := []string{
		"--set=concourse.web.kubernetes.enabled=false",
		"--set=worker.replicas=1",
	}

	deployConcourseChart(releaseName, append(helmDeployTestFlags, selectorFlags...)...)
}

func containerLimitsWork(selectorFlags ...string) {
	Context("container limits work", func() {
		It("returns the configure default container limit", func() {
			deployWithSelectors(selectorFlags...)
			waitAndLogin()

			fly.RunWithRetry("execute", "-c", "tasks/tiny.yml")

			Eventually(func() (*gexec.Session, int) {
				hijackSession := fly.Start(
					"hijack",
					"-b", "1",
					"--", "sh", "-c",
					"cat /sys/fs/cgroup/memory/memory.memsw.limit_in_bytes; cat /sys/fs/cgroup/cpu/cpu.shares",
				)
				<-hijackSession.Exited
				return hijackSession, hijackSession.ExitCode()
			}, 1*time.Minute).Should(gbytes.Say("1073741824\n512"))
			// exitCode is implicitly asserted to be zero value by Gomega
		})
	})
}

func containerLimitsFail(selectorFlags ...string) {
	Context("container limits fail", func() {
		It("fails to set the memory limit", func() {
			deployWithSelectors(selectorFlags...)
			waitAndLogin()
			Eventually(func() *gexec.Session {
				buildSession := fly.Start("execute", "-c", "tasks/tiny.yml")
				<-buildSession.Exited
				return buildSession
			}, 1*time.Minute).Should(gbytes.Say(`(failed to write 1073741824 to memory\.memsw\.limit_in_bytes.*permission denied)`)) // regex checks for inclusion of two strings, which were separately checked in previous test
		})
	})
}

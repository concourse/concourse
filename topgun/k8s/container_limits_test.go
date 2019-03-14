package k8s_test

import (
	"fmt"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garden Config", func() {
	var (
		proxySession        *gexec.Session
		releaseName         string
		namespace           string
		atcEndpoint         string
		nodeImage           string
		helmDeployTestFlags []string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-cl-%d", randomGenerator.Int())
		namespace = releaseName
		Run(nil, "kubectl", "create", "namespace", namespace)
	})

	JustBeforeEach(func() {
		helmDeployTestFlags = []string{
			`--set=worker.replicas=1`,
			`--set=concourse.web.defaultTaskCpuLimit=512`,
			`--set=concourse.web.defaultTaskMemoryLimit=1GB`,
			"--set=worker.nodeSelector.nodeImage=" + nodeImage,
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
	})

	AfterEach(func() {
		helmDestroy(releaseName)
		Wait(Start(nil, "kubectl", "delete", "namespace", namespace, "--wait=false"))
		Wait(proxySession.Interrupt())
	})

	Context("passing a config map location to the worker to be used by gdn", func() {
		BeforeEach(func() {
			nodeImage = "cos"
		})

		It("returns the configure default container limit", func() {
			buildSession := fly.Start("execute", "-c", "../tasks/tiny.yml")
			<-buildSession.Exited
			Expect(buildSession.ExitCode()).To(Equal(0))

			hijackSession := fly.Start(
				"hijack",
				"-b", "1",
				"--", "sh", "-c",
				"cat /sys/fs/cgroup/memory/memory.memsw.limit_in_bytes; cat /sys/fs/cgroup/cpu/cpu.shares",
			)
			<-hijackSession.Exited

			Expect(hijackSession.ExitCode()).To(Equal(0))
			Expect(hijackSession).To(gbytes.Say("1073741824\n512"))
		})
	})

	Context("passing a config map location to the worker to be used by gdn", func() {
		// Context skipped as GKE ubuntu doesn't support container limits.
		BeforeEach(func() {
			nodeImage = "ubuntu"
		})

		It("returns the configure default container limit", func() {
			buildSession := fly.Start("execute", "-c", "../tasks/tiny.yml")
			<-buildSession.Exited
			Expect(buildSession.ExitCode()).To(Equal(2))

			Expect(buildSession).To(gbytes.Say("failed to write 1073741824 to memory.memsw.limit_in_bytes"))
			Expect(buildSession).To(gbytes.Say("permission denied"))
		})
	})

})

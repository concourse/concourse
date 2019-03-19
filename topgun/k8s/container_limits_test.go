package k8s_test

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/concourse/concourse/topgun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container Limits", func() {
	var (
		proxySession        *gexec.Session
		releaseName         string
		namespace           string
		atcEndpoint         string
		nodeImage           string
		helmDeployTestFlags []string
	)

	BeforeEach(func() {
		releaseName = fmt.Sprintf("topgun-cl-%d-%d", rand.Int(), GinkgoParallelNode())
		namespace = releaseName
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

	Context("using cos as NodeImage", func() {
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

	Context("using Ubuntu as NodeImage", func() {
		BeforeEach(func() {
			nodeImage = "ubuntu"
		})

		It("fails to set the memory limit", func() {
			buildSession := fly.Start("execute", "-c", "../tasks/tiny.yml")
			<-buildSession.Exited
			Expect(buildSession.ExitCode()).To(Equal(2))

			Expect(buildSession).To(gbytes.Say("failed to write 1073741824 to memory.memsw.limit_in_bytes"))
			Expect(buildSession).To(gbytes.Say("permission denied"))
		})
	})

})

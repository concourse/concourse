package k8s_test

import (
	"github.com/onsi/gomega/gbytes"

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
		cleanup(releaseName, namespace)
	})

})

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

			atc := waitAndLogin(namespace, releaseName+"-web")
			defer atc.Close()

			buildSession := fly.Start("execute", "-c", "tasks/tiny.yml")
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
}

func containerLimitsFail(selectorFlags ...string) {
	Context("container limits fail", func() {
		It("fails to set the memory limit", func() {
			deployWithSelectors(selectorFlags...)

			atc := waitAndLogin(namespace, releaseName+"-web")
			defer atc.Close()

			buildSession := fly.Start("execute", "-c", "tasks/tiny.yml")
			<-buildSession.Exited
			Expect(buildSession.ExitCode()).To(Equal(2))
			Expect(buildSession).To(gbytes.Say(
				"memory.memsw.limit_in_bytes: permission denied",
			))
		})
	})
}

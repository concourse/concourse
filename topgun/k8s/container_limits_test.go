package k8s_test

import (
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo/v2"
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
		containerLimitsWork(UBUNTU, TaskCPULimit, TaskMemoryLimit)
	})

	AfterEach(func() {
		cleanupReleases()
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
				"-s", "one-off",
				"--",
				"cat",
				"/sys/fs/cgroup/memory/memory.limit_in_bytes",
				"/sys/fs/cgroup/memory.max", // When cgroupsv1 is disabled this file will be present
			)
			<-hijackSession.Exited
			Expect(hijackSession).To(gbytes.Say("1073741824"))

			hijackSession = fly.Start(
				"hijack",
				"-b", "1",
				"-s", "one-off",
				"--",
				"cat",
				"/sys/fs/cgroup/cpu/cpu.shares",
				"/sys/fs/cgroup/cpu.weight", // When cgroupsv1 is disabled this file will be present
			)
			<-hijackSession.Exited

			// Note: todo copied from https://github.com/concourse/concourse/blob/754cc9909e931dd0b0ba7be808a788f96a98a44c/testflight/container_limits_test.go#L70
			// TODO: This is 20 in cgroups v2. Not sure why this happens though. It
			// is being set and the value changes if we set CPU to higher or lower
			// values. Can't figure out how it determines what to change the value
			// to. Doesn't look like we change it at all, so must be someting in
			// containerd or runc. Not sure if its related to whatever value is in
			// the parent cgroup
			Expect(hijackSession).To(Or(gbytes.Say("512"), gbytes.Say("20")))
		})
	})
}

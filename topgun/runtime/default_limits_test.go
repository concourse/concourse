package topgun_test

import (
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("An ATC with default resource limits set", func() {
	BeforeEach(func() {
		Deploy(
			"deployments/concourse.yml",
			"-o", "operations/default_limits.yml",
			"-v", "default_task_cpu_limit=512",
			"-v", "default_task_memory_limit=1GB",
		)
	})

	It("respects the default resource limits, overridding when specified", func() {
		buildSession := Fly.Start("execute", "-c", "tasks/tiny.yml")
		<-buildSession.Exited

		interceptSession := Fly.Start(
			"intercept",
			"-b", "1",
			"-s", "one-off",
			"--", "sh", "-c",
			"cat /sys/fs/cgroup/memory/memory.memsw.limit_in_bytes; cat /sys/fs/cgroup/cpu/cpu.shares",
		)
		<-interceptSession.Exited

		Expect(interceptSession.ExitCode()).To(Equal(0))
		Expect(interceptSession).To(gbytes.Say("1073741824\n512"))

		buildSession = Fly.Start("execute", "-c", "tasks/limits.yml")
		<-buildSession.Exited

		interceptSession = Fly.Start(
			"intercept",
			"-b", "2",
			"-s", "one-off",
			"--", "sh", "-c",
			"cat /sys/fs/cgroup/memory/memory.memsw.limit_in_bytes; cat /sys/fs/cgroup/cpu/cpu.shares",
		)
		<-interceptSession.Exited

		Expect(interceptSession.ExitCode()).To(Equal(0))
		Expect(interceptSession).To(gbytes.Say("104857600\n256"))
	})
})

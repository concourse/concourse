package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that has container limits", func() {

	It("successfully runs the build", func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/container_limits.yml",
		)

		watch := flyHelper.TriggerJob(pipelineName, "container-limits-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("hello"))
	})

	It("sets the correct CPU and memory limits on the container", func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/container_limits_failing.yml",
		)

		watch := flyHelper.TriggerJob(pipelineName, "container-limits-failing-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("failed"))
		Expect(watch).To(gexec.Exit(1))

		hijackS := flyHelper.Hijack(
			"-j", pipelineName+"/container-limits-failing-job",
			"--", "sh", "-c",
			"cat /sys/fs/cgroup/memory/memory.memsw.limit_in_bytes; cat /sys/fs/cgroup/cpu/cpu.shares")
		<-hijackS.Exited

		Eventually(Expect(hijackS).To(gbytes.Say("1073741824")))
		Eventually(Expect(hijackS).To(gbytes.Say("512")))

		Expect(hijackS).To(gexec.Exit(0))
	})
})

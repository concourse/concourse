package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that has container limits", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/container_limits.yml",
		)
	})

	It("successfully runs the build in a container with the correct CPU and memory limits", func() {
		watch := flyHelper.TriggerJob(pipelineName, "container-limits-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("hello"))

		hijackS := flyHelper.Hijack(
			"-j", pipelineName+"/container-limits-job",
			"--", "sh", "-c",
			"cat /sys/fs/cgroup/memory/memory.memsw.limit_in_bytes; cat /sys/fs/cgroup/cpu/cpu.shares")
		<-hijackS.Exited
		Eventually(Expect(hijackS).To(gbytes.Say("1073741824")))
		Eventually(Expect(hijackS).To(gbytes.Say("512")))

		Expect(hijackS).To(gexec.Exit(0))
		Expect(watch).To(gexec.Exit(0))
	})
})

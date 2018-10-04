package testflight_test

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that has container limits", func() {
	It("successfully runs the build", func() {
		setAndUnpausePipeline("fixtures/container_limits.yml")

		watch := spawnFly("trigger-job", "-j", inPipeline("container-limits-job"), "-w")
		<-watch.Exited

		if strings.Contains(string(watch.Out.Contents()), "memsw.limit_in_bytes: permission denied") {
			Skip("swap limits not enabled; skipping")
		}

		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("hello"))
	})

	It("sets the correct CPU and memory limits on the container", func() {
		setAndUnpausePipeline("fixtures/container_limits_failing.yml")

		watch := spawnFly("trigger-job", "-j", inPipeline("container-limits-failing-job"), "-w")
		<-watch.Exited

		if strings.Contains(string(watch.Out.Contents()), "memsw.limit_in_bytes: permission denied") {
			Skip("swap limits not enabled; skipping")
		}

		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("failed"))
		Expect(watch).To(gexec.Exit(1))

		interceptS := fly("intercept", "-j", inPipeline("container-limits-failing-job"),
			"--",
			"cat",
			"/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes",
		)
		Expect(interceptS).To(gbytes.Say("1073741824"))

		interceptS = fly("intercept", "-j", inPipeline("container-limits-failing-job"),
			"--",
			"cat",
			"/sys/fs/cgroup/cpu/cpu.shares",
		)
		Expect(interceptS).To(gbytes.Say("512"))
	})
})

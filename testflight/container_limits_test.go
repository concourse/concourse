package testflight_test

import (
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that has container limits", func() {
	It("successfully runs the build", func() {
		setAndUnpausePipeline("fixtures/container_limits.yml")

		watch := spawnFly("trigger-job", "-j", inPipeline("container-limits-job"), "-w")
		<-watch.Exited

		match, _ := regexp.MatchString(`open /sys/fs/cgroup/memory/.*/memory\.memsw\.limit_in_bytes: (permission denied)?(no such file or directory)?`,
			string(watch.Out.Contents()))

		// Guardian runtime will always fail this test unless it's explicitly told not to set the swap limit
		if match {
			Skip("swap limits not enabled; skipping")
		}

		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("hello"))
	})

	It("sets the correct CPU and memory limits on the container", func() {
		setAndUnpausePipeline("fixtures/container_limits_failing.yml")

		watch := spawnFly("trigger-job", "-j", inPipeline("container-limits-failing-job"), "-w")
		<-watch.Exited

		match, _ := regexp.MatchString(`open /sys/fs/cgroup/memory/.*/memory\.memsw\.limit_in_bytes: (permission denied)?(no such file or directory)?`,
			string(watch.Out.Contents()))

		// Guardian runtime will always fail this test unless it's explicitly told not to set the swap limit
		if match {
			Skip("swap limits not enabled; skipping")
		}

		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("failed"))
		Expect(watch).To(gexec.Exit(1))

		interceptS := fly(
			"intercept",
			"-j", inPipeline("container-limits-failing-job"),
			"-s", "task-with-container-limits",
			"--",
			"cat",
			"/sys/fs/cgroup/memory/memory.limit_in_bytes",
		)
		Expect(interceptS).To(gbytes.Say("1073741824"))

		interceptS = fly(
			"intercept",
			"-j", inPipeline("container-limits-failing-job"),
			"-s", "task-with-container-limits",
			"--",
			"cat",
			"/sys/fs/cgroup/cpu/cpu.shares",
		)
		Expect(interceptS).To(gbytes.Say("512"))
	})
})

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

		interceptS := flyUnsafe(
			"intercept",
			"-j", inPipeline("container-limits-failing-job"),
			"-s", "task-with-container-limits",
			"--",
			"cat",
			"/sys/fs/cgroup/memory/memory.limit_in_bytes",
			"/sys/fs/cgroup/memory.max", // When cgroupsv1 is disabled this file will be present
		)
		Expect(interceptS).To(gbytes.Say("1073741824"))

		interceptS = flyUnsafe(
			"intercept",
			"-j", inPipeline("container-limits-failing-job"),
			"-s", "task-with-container-limits",
			"--",
			"cat",
			"/sys/fs/cgroup/cpu/cpu.shares",
			"/sys/fs/cgroup/cpu.weight", // When cgroupsv1 is disabled this file will be present
		)

		// TODO: This is 20 in cgroups v2. Not sure why this happens though. It
		// is being set and the value changes if we set CPU to higher or lower
		// values. Can't figure out how it determines what to change the value
		// to. Doesn't look like we change it at all, so must be someting in
		// containerd or runc. Not sure if its related to whatever value is in
		// the parent cgroup
		Expect(interceptS).To(Or(gbytes.Say("512"), gbytes.Say("20")))
	})
})

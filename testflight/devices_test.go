package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Devices", func() {
	It("mounts the /dev/fuse device to privileged containers", func() {
		setAndUnpausePipeline("fixtures/devices.yml")

		watch := fly("trigger-job", "-j", inPipeline("check-fuse-privileged"), "-w")
		Expect(watch).To(gbytes.Say("succeeded"))
	})

	It("does not mount the /dev/fuse device to unprivileged containers", func() {
		setAndUnpausePipeline("fixtures/devices.yml")

		watch := flyUnsafe("trigger-job", "-j", inPipeline("check-fuse-unprivileged"), "-w")
		Expect(watch.ExitCode()).To(Equal(1))
		Expect(watch).To(gbytes.Say("No such file or directory"))
	})
})

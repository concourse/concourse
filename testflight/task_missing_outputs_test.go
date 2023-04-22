package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A task with no outputs declared", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/task-missing-outputs.yml")
	})

	It("doesn't mount its file system into the next task", func() {
		watch := spawnFly("trigger-job", "-j", inPipeline("missing-outputs-job"), "-w")
		<-watch.Exited
		Expect(watch).To(gexec.Exit(2))
		Expect(watch).To(gbytes.Say("missing inputs: missing-outputs"))
	})
})

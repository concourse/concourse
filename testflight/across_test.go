package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A pipeline containing an across step", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/across.yml")
	})

	It("performs the across steps", func() {
		watch := fly("trigger-job", "-j", inPipeline("job"), "-w")

		Expect(watch).To(gbytes.Say("running across static1 dynamic1"))
		Expect(watch).To(gbytes.Say("running across static1 dynamic2"))
		Expect(watch).To(gbytes.Say("running across static1 dynamic3"))
		Expect(watch).To(gbytes.Say("running across static1 4"))
		Expect(watch).To(gbytes.Say("running across static2 dynamic1"))
		Expect(watch).To(gbytes.Say("running across static2 dynamic2"))
		Expect(watch).To(gbytes.Say("running across static2 dynamic3"))
		Expect(watch).To(gbytes.Say("running across static2 4"))

		Expect(watch).To(gbytes.Say("pushing version: v_dynamic1"))
		Expect(watch).To(gbytes.Say("fetching version: v_dynamic1"))
		Expect(watch).To(gbytes.Say("pushing version: v_dynamic2"))
		Expect(watch).To(gbytes.Say("fetching version: v_dynamic2"))
		Expect(watch).To(gbytes.Say("pushing version: v_dynamic3"))
		Expect(watch).To(gbytes.Say("fetching version: v_dynamic3"))
		Expect(watch).To(gbytes.Say("pushing version: v_4"))
		Expect(watch).To(gbytes.Say("fetching version: v_4"))
	})
})

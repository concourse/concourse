package atc_test

import (
	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Context("FindBaseResourceTypeDefaults", func() {
	BeforeEach(func() {
		atc.LoadBaseResourceTypeDefaults(map[string]atc.Source{
			"rt-a": atc.Source{
				"k1": "v1",
			},
			"rt-b": atc.Source{
				"k2": "v2",
			},
		})
	})
	AfterEach(func() {
		atc.LoadBaseResourceTypeDefaults(map[string]atc.Source{})
	})

	It("should find defined defaults", func() {
		a, found := atc.FindBaseResourceTypeDefaults("rt-a")
		Expect(found).To(BeTrue())
		Expect(a).To(Equal(atc.Source{"k1": "v1"}))

		b, found := atc.FindBaseResourceTypeDefaults("rt-b")
		Expect(found).To(BeTrue())
		Expect(b).To(Equal(atc.Source{"k2": "v2"}))

		_, found = atc.FindBaseResourceTypeDefaults("rt-c")
		Expect(found).To(BeFalse())
	})
})

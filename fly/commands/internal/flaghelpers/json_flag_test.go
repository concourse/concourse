package flaghelpers_test

import (
	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/fly/commands/internal/flaghelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JsonFlag", func() {
	Context("when JSON string is invalid", func() {
		It("displays an error message", func() {
			jsonFlag := &JsonFlag{}

			err := jsonFlag.UnmarshalFlag("{some:value}")
			Expect(err).To(MatchError("invalid character 's' looking for beginning of object key string"))
		})
	})

	Context("when JSON string is valid", func() {
		It("parse the JSON into version", func() {
			jsonFlag := &JsonFlag{}

			err := jsonFlag.UnmarshalFlag(`{"some":"value"}`)
			Expect(err).ToNot(HaveOccurred())
			Expect(jsonFlag.Version).To(Equal(atc.Version{"some": "value"}))
			Expect(jsonFlag.JsonString).To(Equal(`{"some":"value"}`))
		})
	})
})

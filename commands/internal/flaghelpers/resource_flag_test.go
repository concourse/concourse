package flaghelpers_test

import (
	. "github.com/concourse/fly/commands/internal/flaghelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceFlag", func() {
	Context("when there is only a pipeline specified", func() {
		It("displays an error message", func() {
			resourceFlag := &ResourceFlag{}

			err := resourceFlag.UnmarshalFlag("pipeline")
			Expect(err).To(MatchError("argument format should be <pipeline>/<resource>"))
		})
	})
})

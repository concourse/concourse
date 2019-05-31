package flaghelpers_test

import (
	. "github.com/concourse/concourse/v5/fly/commands/internal/flaghelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobFlag", func() {
	Context("when there is only a pipeline specified", func() {
		It("displays an error message", func() {
			jobFlag := &JobFlag{}

			err := jobFlag.UnmarshalFlag("pipeline")
			Expect(err).To(MatchError("argument format should be <pipeline>/<job>"))
		})
	})
})

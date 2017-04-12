package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline", func() {
	var (
		pipeline dbng.Pipeline
		err      error
	)

	BeforeEach(func() {
		pipeline, _, err = defaultTeam.SavePipeline("fake-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "job-name"},
			},
		}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Hide", func() {
		JustBeforeEach(func() {
			err = pipeline.Hide()
			Expect(err).ToNot(HaveOccurred())

			found, err := pipeline.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when the pipeline is public", func() {
			BeforeEach(func() {
				err = pipeline.Expose()
				Expect(err).ToNot(HaveOccurred())
			})

			It("sets public to be false", func() {
				Expect(pipeline.Public()).To(BeFalse())
			})
		})
	})
})

package atc_test

import (
	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var config atc.Config

	Describe("determining if a job's builds are publically viewable", func() {
		Context("when the job is publically viewable", func() {
			BeforeEach(func() {
				config = atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name:   "some-job",
							Public: true,
						},
					},
				}
			})

			It("returns true", func() {
				public, _ := config.JobIsPublic("some-job")
				Expect(public).To(BeTrue())
			})

			It("does not error", func() {
				_, err := config.JobIsPublic("some-job")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the job is not publically viewable", func() {
			BeforeEach(func() {
				config = atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name:   "some-job",
							Public: false,
						},
					},
				}
			})

			It("returns false", func() {
				public, _ := config.JobIsPublic("some-job")
				Expect(public).To(BeFalse())
			})

			It("does not error", func() {
				_, err := config.JobIsPublic("some-job")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the job with the given name can't be found", func() {
			BeforeEach(func() {
				config = atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name:   "some-job",
							Public: false,
						},
					},
				}
			})

			It("errors", func() {
				_, err := config.JobIsPublic("does-not-exist")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

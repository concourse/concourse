package dbng_test

import (
	"github.com/concourse/atc/dbng"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("BuildFactory", func() {
	Describe("MarkNonInterceptibleBuilds", func() {
		Context("one-off builds", func() {
			DescribeTable("completed builds",
				func(status dbng.BuildStatus, matcher types.GomegaMatcher) {
					b, err := defaultTeam.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())

					var i bool
					b.Finish(status)

					err = buildFactory.MarkNonInterceptibleBuilds()
					Expect(err).NotTo(HaveOccurred())

					i, err = b.Interceptible()
					Expect(err).NotTo(HaveOccurred())
					Expect(i).To(matcher)
				},
				Entry("succeeded is non-interceptible", dbng.BuildStatusSucceeded, BeFalse()),
				Entry("aborted is non-interceptible", dbng.BuildStatusAborted, BeFalse()),
				Entry("errored is non-interceptible", dbng.BuildStatusErrored, BeFalse()),
				Entry("failed is non-interceptible", dbng.BuildStatusFailed, BeFalse()),
			)

			It("non-completed is interceptible", func() {
				b, err := defaultTeam.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())

				var i bool
				err = buildFactory.MarkNonInterceptibleBuilds()
				i, err = b.Interceptible()
				Expect(err).NotTo(HaveOccurred())
				Expect(i).To(BeTrue())
			})
		})

		Context("pipeline builds", func() {

			It("[#139963615] marks builds that aren't the latest as non-interceptible, ", func() {
				build1, err := defaultPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				build2, err := defaultPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				build1.Finish(dbng.BuildStatusErrored)
				build2.Finish(dbng.BuildStatusErrored)

				p, _, err := defaultTeam.SavePipeline("other-pipeline", atc.Config{
					Jobs: atc.JobConfigs{
						{
							Name: "some-other-job",
						},
					},
				}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				pb1, err := p.CreateJobBuild("some-other-job")
				Expect(err).NotTo(HaveOccurred())

				pb2, err := p.CreateJobBuild("some-other-job")
				Expect(err).NotTo(HaveOccurred())

				pb1.Finish(dbng.BuildStatusErrored)
				pb2.Finish(dbng.BuildStatusErrored)

				err = buildFactory.MarkNonInterceptibleBuilds()
				Expect(err).NotTo(HaveOccurred())

				var i bool
				i, err = build1.Interceptible()
				Expect(err).NotTo(HaveOccurred())
				Expect(i).To(BeFalse())

				i, err = build2.Interceptible()
				Expect(err).NotTo(HaveOccurred())
				Expect(i).To(BeTrue())

				i, err = pb1.Interceptible()
				Expect(err).NotTo(HaveOccurred())
				Expect(i).To(BeFalse())

				i, err = pb2.Interceptible()
				Expect(err).NotTo(HaveOccurred())
				Expect(i).To(BeTrue())

			})

			DescribeTable("completed builds",
				func(status dbng.BuildStatus, matcher types.GomegaMatcher) {
					b, err := defaultPipeline.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())

					var i bool
					b.Finish(status)
					err = buildFactory.MarkNonInterceptibleBuilds()
					i, err = b.Interceptible()
					Expect(err).NotTo(HaveOccurred())
					Expect(i).To(matcher)
				},
				Entry("succeeded is non-interceptible", dbng.BuildStatusSucceeded, BeFalse()),
				Entry("aborted is interceptible", dbng.BuildStatusAborted, BeTrue()),
				Entry("errored is interceptible", dbng.BuildStatusErrored, BeTrue()),
				Entry("failed is interceptible", dbng.BuildStatusFailed, BeTrue()),
			)

			It("does not mark non-completed builds", func() {
				b, err := defaultPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				var i bool
				i, err = b.Interceptible()
				Expect(err).NotTo(HaveOccurred())
				Expect(i).To(BeTrue())

				err = buildFactory.MarkNonInterceptibleBuilds()
				i, err = b.Interceptible()
				Expect(err).NotTo(HaveOccurred())
				Expect(i).To(BeTrue())

				err = b.SaveStatus(dbng.BuildStatusStarted)
				Expect(err).NotTo(HaveOccurred())

				err = buildFactory.MarkNonInterceptibleBuilds()
				i, err = b.Interceptible()
				Expect(err).NotTo(HaveOccurred())
				Expect(i).To(BeTrue())
			})
		})
	})
})

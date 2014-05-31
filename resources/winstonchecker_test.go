package resources_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/redisrunner"
	. "github.com/winston-ci/winston/resources"
)

var _ = Describe("WinstonChecker", func() {
	var redisRunner *redisrunner.Runner

	var redis db.DB
	var jobs []string
	var checker Checker

	var resource config.Resource

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		jobs = nil

		resource = config.Resource{
			Name:   "some-resource",
			Type:   "git",
			Source: config.Source{"some": "starting-source"},
		}
	})

	AfterEach(func() {
		redisRunner.Stop()
	})

	JustBeforeEach(func() {
		checker = NewWinstonChecker(redis, jobs)
	})

	Context("with one job", func() {
		BeforeEach(func() {
			jobs = []string{"job1"}
		})

		Context("when the job does not have the resource as an output", func() {
			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource, nil)).Should(BeEmpty())
			})
		})
	})

	Context("when depending on multiple jobs", func() {
		BeforeEach(func() {
			jobs = []string{"job1", "job2"}
		})

		Context("when neither job has a output", func() {
			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource, nil)).Should(BeEmpty())
			})
		})

		Context("when only one job has output", func() {
			BeforeEach(func() {
				err := redis.SaveOutputVersion("job1", 1, "some-resource", builds.Version{"version": "123"})
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource, nil)).Should(BeEmpty())
			})
		})

		Context("when both jobs have common output", func() {
			BeforeEach(func() {
				err := redis.SaveOutputVersion("job1", 1, "some-resource", builds.Version{"version": "123"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job2", 2, "some-resource", builds.Version{"version": "123"})
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns the common version", func() {
				Ω(checker.CheckResource(resource, nil)).Should(Equal([]builds.Version{{"version": "123"}}))
			})
		})

		Context("when both jobs have common output, including the given version", func() {
			BeforeEach(func() {
				err := redis.SaveOutputVersion("job1", 1, "some-resource", builds.Version{"version": "old"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job2", 1, "some-resource", builds.Version{"version": "old"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job1", 2, "some-resource", builds.Version{"version": "current"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job2", 2, "some-resource", builds.Version{"version": "current"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job1", 3, "some-resource", builds.Version{"version": "new"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job2", 3, "some-resource", builds.Version{"version": "new"})
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not include the starting source or any sources before it", func() {
				Ω(checker.CheckResource(resource, builds.Version{"version": "current"})).Should(Equal([]builds.Version{
					builds.Version{"version": "new"},
				}))
			})
		})

		Context("when the jobs both have the given version as their most recent common output", func() {
			BeforeEach(func() {
				err := redis.SaveOutputVersion("job1", 1, "some-resource", builds.Version{"version": "old"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job2", 1, "some-resource", builds.Version{"version": "old"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job1", 2, "some-resource", builds.Version{"version": "current"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job2", 2, "some-resource", builds.Version{"version": "current"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job1", 3, "some-resource", builds.Version{"version": "new"})
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource, builds.Version{"version": "current"})).Should(BeEmpty())
			})
		})

		Context("when the jobs do not have a common output", func() {
			BeforeEach(func() {
				err := redis.SaveOutputVersion("job1", 1, "some-resource", builds.Version{"version": "123"})
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputVersion("job2", 2, "some-resource", builds.Version{"version": "456"})
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource, builds.Version{"version": "old"})).Should(BeEmpty())
			})
		})
	})
})

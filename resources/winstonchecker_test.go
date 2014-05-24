package resources_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			Source: config.Source("starting-source"),
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
				Ω(checker.CheckResource(resource)).Should(BeEmpty())
			})
		})
	})

	Context("with multiple jobs", func() {
		BeforeEach(func() {
			jobs = []string{"job1", "job2"}
		})

		Context("when neither job has a output", func() {
			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource)).Should(BeEmpty())
			})
		})

		Context("when one job has output", func() {
			BeforeEach(func() {
				err := redis.SaveOutputSource("job1", 1, "some-resource", config.Source("123"))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource)).Should(BeEmpty())
			})
		})

		Context("when both jobs have common output", func() {
			BeforeEach(func() {
				err := redis.SaveOutputSource("job1", 1, "some-resource", config.Source("123"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job2", 2, "some-resource", config.Source("123"))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns the common source", func() {
				commonResource := resource
				commonResource.Source = config.Source("123")

				Ω(checker.CheckResource(resource)).Should(Equal([]config.Resource{commonResource}))
			})
		})

		Context("when both jobs have common output, including the given resource", func() {
			BeforeEach(func() {
				err := redis.SaveOutputSource("job1", 1, "some-resource", config.Source("1"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job2", 1, "some-resource", config.Source("1"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job1", 2, "some-resource", config.Source("starting-source"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job2", 2, "some-resource", config.Source("starting-source"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job1", 3, "some-resource", config.Source("3"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job2", 3, "some-resource", config.Source("3"))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("does not include the starting source or any sources before it", func() {
				commonResource := resource
				commonResource.Source = config.Source("1")

				Ω(checker.CheckResource(resource)).Should(Equal([]config.Resource{
					{
						Name:   "some-resource",
						Type:   "git",
						Source: config.Source("3"),
					},
				}))
			})
		})

		Context("when the jobs both have the given resource as their most recent common output", func() {
			BeforeEach(func() {
				err := redis.SaveOutputSource("job1", 1, "some-resource", config.Source("1"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job2", 1, "some-resource", config.Source("1"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job1", 2, "some-resource", config.Source("starting-source"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job2", 2, "some-resource", config.Source("starting-source"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job1", 3, "some-resource", config.Source("3"))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource)).Should(BeEmpty())
			})
		})

		Context("when the jobs do not have a common output", func() {
			BeforeEach(func() {
				err := redis.SaveOutputSource("job1", 1, "some-resource", config.Source("123"))
				Ω(err).ShouldNot(HaveOccurred())

				err = redis.SaveOutputSource("job2", 2, "some-resource", config.Source("456"))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns an empty slice", func() {
				Ω(checker.CheckResource(resource)).Should(BeEmpty())
			})
		})
	})
})

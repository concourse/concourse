package queue_test

import (
	"errors"
	"time"
	"github.com/winston-ci/winston/builder/fakebuilder"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	. "github.com/winston-ci/winston/queue"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Queue", func() {

	var gracePeriod time.Duration
	var builder *fakebuilder.FakeBuilder
	var queue *Queue

	var job config.Job
	var resource config.Resource
	var version builds.Version

	var createdBuild builds.Build

	BeforeEach(func() {
		gracePeriod = 100 * time.Millisecond
		builder = new(fakebuilder.FakeBuilder)

		job = config.Job{
			Name: "some-job",
		}

		resource = config.Resource{
			Name: "some-resource",
		}

		version = builds.Version{"ver": "1"}

		createdBuild = builds.Build{
			ID:     42,
			Status: builds.StatusPending,
		}

		queue = NewQueue(gracePeriod, builder)
	})

	Context("when a build is triggered", func() {
		BeforeEach(func() {
			builder.CreateReturns(createdBuild, nil)
		})

		It("creates and executes the build immediately", func() {
			triggeredBuild, err := queue.Trigger(job)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(triggeredBuild).Should(Equal(createdBuild))

			Ω(builder.CreateCallCount()).Should(Equal(1))

			createdJob := builder.CreateArgsForCall(0)
			Ω(createdJob).Should(Equal(job))

			Ω(builder.StartCallCount()).Should(Equal(1))

			startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
			Ω(startedJob).Should(Equal(job))
			Ω(startedBuild).Should(Equal(triggeredBuild))
			Ω(startedVersions).Should(BeNil())
		})

		Context("when the build still cannot be started", func() {
			BeforeEach(func() {
				builder.StartReturns(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				}, nil)
			})

			It("tries yet again after the grace period", func() {
				queuedBuild, err := queue.Trigger(job)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(queuedBuild).Should(Equal(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				}))

				Eventually(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))

				startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
				Ω(startedBuild).Should(Equal(queuedBuild))
				Ω(startedJob).Should(Equal(job))
				Ω(startedVersions).Should(BeEmpty())

				Eventually(builder.StartCallCount, 2*gracePeriod).Should(Equal(2))

				startedJob, startedBuild, startedVersions = builder.StartArgsForCall(1)
				Ω(startedBuild).Should(Equal(queuedBuild))
				Ω(startedJob).Should(Equal(job))
				Ω(startedVersions).Should(BeEmpty())
			})
		})

		Context("when starting the build fails", func() {
			BeforeEach(func() {
				builder.StartReturns(builds.Build{}, errors.New("oh no!"))
			})

			It("gives up", func() {
				queuedBuild, err := queue.Trigger(job)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(queuedBuild).Should(Equal(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				}))

				Eventually(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))
				Consistently(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))
			})
		})
	})

	Context("when a build is enqueued", func() {
		BeforeEach(func() {
			builder.AttemptReturns(createdBuild, nil)
		})

		It("attempts and executes the build immediately", func() {
			triggeredBuild, err := queue.Enqueue(job, resource, version)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(triggeredBuild).Should(Equal(createdBuild))

			Ω(builder.AttemptCallCount()).Should(Equal(1))

			attemptedJob, attemptedResource, attemptedVersion := builder.AttemptArgsForCall(0)
			Ω(attemptedJob).Should(Equal(job))
			Ω(attemptedResource).Should(Equal(resource))
			Ω(attemptedVersion).Should(Equal(version))

			Ω(builder.StartCallCount()).Should(Equal(1))

			startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
			Ω(startedJob).Should(Equal(job))
			Ω(startedBuild).Should(Equal(triggeredBuild))
			Ω(startedVersions).Should(Equal(map[string]builds.Version{
				resource.Name: version,
			}))
		})

		Context("when the build still cannot be started", func() {
			BeforeEach(func() {
				builder.StartReturns(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				}, nil)
			})

			It("tries yet again after the grace period", func() {
				queuedBuild, err := queue.Enqueue(job, resource, version)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(queuedBuild).Should(Equal(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				}))

				Eventually(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))

				startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
				Ω(startedBuild).Should(Equal(queuedBuild))
				Ω(startedJob).Should(Equal(job))
				Ω(startedVersions).Should(Equal(map[string]builds.Version{
					resource.Name: version,
				}))

				Eventually(builder.StartCallCount, 2*gracePeriod).Should(Equal(2))

				startedJob, startedBuild, startedVersions = builder.StartArgsForCall(1)
				Ω(startedBuild).Should(Equal(queuedBuild))
				Ω(startedJob).Should(Equal(job))
				Ω(startedVersions).Should(Equal(map[string]builds.Version{
					resource.Name: version,
				}))
			})
		})

		Context("when starting the build fails", func() {
			BeforeEach(func() {
				builder.StartReturns(builds.Build{}, errors.New("oh no!"))
			})

			It("gives up", func() {
				queuedBuild, err := queue.Enqueue(job, resource, version)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(queuedBuild).Should(Equal(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				}))

				Eventually(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))
				Consistently(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))
			})
		})
	})
})

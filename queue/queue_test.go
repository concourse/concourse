package queue_test

import (
	"errors"
	"os"
	"time"

	"github.com/tedsuo/ifrit"
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

	var process ifrit.Process

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

		createdBuild = builds.Build{ID: 42}

		builder.CreateReturns(createdBuild, nil)

		queue = NewQueue(gracePeriod, builder)

		process = ifrit.Envoke(queue)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive(BeNil()))
	})

	Context("when a build is triggered", func() {
		var startedAt time.Time

		BeforeEach(func() {
			startedAt = time.Now()
		})

		It("executes the build immediately", func() {
			startedBuild, _ := queue.Trigger(job)
			Eventually(startedBuild, 2*gracePeriod).Should(Receive(Equal(builds.Build{
				ID: 42,
			})))

			Ω(time.Since(startedAt)).Should(BeNumerically("<", gracePeriod))

			Ω(builder.CreateCallCount()).Should(Equal(1))

			createdJob := builder.CreateArgsForCall(0)
			Ω(createdJob).Should(Equal(job))
		})

		Context("when the new build's status is pending", func() {
			BeforeEach(func() {
				builder.CreateReturns(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				}, nil)
			})

			It("returns the queued build", func() {
				queuedBuild, _ := queue.Trigger(job)
				Eventually(queuedBuild, 2*gracePeriod).Should(Receive(Equal(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				})))
			})

			It("tries starting the build after the grace period", func() {
				gotPending, _ := queue.Trigger(job)

				var queuedBuild builds.Build
				Eventually(gotPending, 2*gracePeriod).Should(Receive(&queuedBuild))
				Ω(queuedBuild).Should(Equal(builds.Build{
					ID:     42,
					Status: builds.StatusPending,
				}))

				Eventually(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))

				startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
				Ω(startedBuild).Should(Equal(queuedBuild))
				Ω(startedJob).Should(Equal(job))
				Ω(startedVersions).Should(BeEmpty())
			})

			Context("when the build still cannot be started", func() {
				BeforeEach(func() {
					builder.StartReturns(builds.Build{
						ID:     42,
						Status: builds.StatusPending,
					}, nil)
				})

				It("tries yet again after the grace period", func() {
					gotPending, _ := queue.Trigger(job)

					var queuedBuild builds.Build
					Eventually(gotPending, 2*gracePeriod).Should(Receive(&queuedBuild))
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
					queuedBuild, _ := queue.Trigger(job)

					Eventually(queuedBuild, 2*gracePeriod).Should(Receive(Equal(builds.Build{
						ID:     42,
						Status: builds.StatusPending,
					})))

					Eventually(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))
					Consistently(builder.StartCallCount, 2*gracePeriod).Should(Equal(1))
				})
			})
		})

		Context("and the queue is told to stop", func() {
			BeforeEach(func() {
				builder.CreateStub = func(job config.Job) (builds.Build, error) {
					return builds.Build{ID: 42}, nil
				}
			})

			It("waits for the build to start", func() {
				startedBuild, _ := queue.Trigger(job)

				process.Signal(os.Interrupt)

				select {
				case build := <-startedBuild:
					Ω(build).Should(Equal(builds.Build{ID: 42}))
				case <-process.Wait():
					Fail("should have gotten the started build before exiting!")
				}

				Eventually(process.Wait()).Should(Receive(BeNil()))
			})
		})
	})

	Context("when a build is enqueued", func() {
		var startedAt time.Time
		var startedBuild <-chan builds.Build
		var queueErr <-chan error

		BeforeEach(func() {
			startedAt = time.Now()

			startedBuild, queueErr = queue.Enqueue(job, resource, version)
		})

		It("starts a build after the elapsed grace period", func() {
			Eventually(startedBuild, 2*gracePeriod).Should(Receive())

			Ω(time.Since(startedAt)).Should(BeNumerically("~", gracePeriod, gracePeriod/2))

			Ω(builder.StartCallCount()).Should(Equal(1))

			startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
			Ω(startedJob).Should(Equal(job))
			Ω(startedBuild).Should(Equal(createdBuild))
			Ω(startedVersions).Should(Equal(map[string]builds.Version{
				"some-resource": builds.Version{"ver": "1"},
			}))
		})

		Context("and the queue is told to stop", func() {
			BeforeEach(func() {
				process.Signal(os.Interrupt)
			})

			It("waits for the builds to be triggered", func(done Done) {
				defer close(done)

				select {
				case <-startedBuild:
				case <-process.Wait():
					Fail("should have gotten the started build before exiting!")
				}
			}, 2.0)
		})

		Context("when building fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				builder.CreateReturns(builds.Build{}, disaster)
			})

			It("emits the error", func() {
				Eventually(queueErr, 2*gracePeriod).Should(Receive(Equal(disaster)))
			})
		})

		Context("and it is then triggered", func() {
			It("executes the queued build immediately", func() {
				preemptedBuild, _ := queue.Trigger(job)

				Ω(<-startedBuild).Should(Equal(<-preemptedBuild))

				Ω(time.Since(startedAt)).Should(BeNumerically("<", gracePeriod))

				Ω(builder.StartCallCount()).Should(Equal(1))

				startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
				Ω(startedJob).Should(Equal(job))
				Ω(startedBuild).Should(Equal(createdBuild))
				Ω(startedVersions).Should(Equal(map[string]builds.Version{
					"some-resource": builds.Version{"ver": "1"},
				}))
			})
		})

		Context("and another build of the job is enqueued", func() {
			var (
				secondResource config.Resource
				secondVersion  builds.Version

				secondQueuedBuild <-chan builds.Build
				secondQueueErr    <-chan error
			)

			BeforeEach(func() {
				secondResource = config.Resource{
					Name:   "second-resource",
					Type:   "git",
					Source: config.Source{"uri": "http://second-resource"},
				}

				secondVersion = builds.Version{"ver": "2"}
			})

			Context("within the grace period", func() {
				BeforeEach(func() {
					secondQueuedBuild, secondQueueErr = queue.Enqueue(job, secondResource, secondVersion)
				})

				It("starts the build with both specified versions", func() {
					Eventually(secondQueuedBuild, 2*gracePeriod).Should(Receive())

					Eventually(builder.StartCallCount).Should(Equal(1))

					startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
					Ω(startedJob).Should(Equal(job))
					Ω(startedBuild).Should(Equal(createdBuild))
					Ω(startedVersions).Should(Equal(map[string]builds.Version{
						"some-resource":   builds.Version{"ver": "1"},
						"second-resource": builds.Version{"ver": "2"},
					}))
				})
			})

			Context("after the first one completes", func() {
				BeforeEach(func() {
					Eventually(startedBuild, 2*gracePeriod).Should(Receive())

					secondQueuedBuild, secondQueueErr = queue.Enqueue(job, secondResource, secondVersion)
				})

				It("executes a second, separate build with only the new version", func() {
					Eventually(secondQueuedBuild, 2*gracePeriod).Should(Receive())

					Eventually(builder.StartCallCount).Should(Equal(2))

					startedJob, startedBuild, startedVersions := builder.StartArgsForCall(0)
					Ω(startedJob).Should(Equal(job))
					Ω(startedBuild).Should(Equal(createdBuild))
					Ω(startedVersions).Should(Equal(map[string]builds.Version{
						"some-resource": builds.Version{"ver": "1"},
					}))

					startedJob, startedBuild, startedVersions = builder.StartArgsForCall(1)
					Ω(startedJob).Should(Equal(job))
					Ω(startedBuild).Should(Equal(createdBuild))
					Ω(startedVersions).Should(Equal(map[string]builds.Version{
						"second-resource": builds.Version{"ver": "2"},
					}))
				})
			})
		})
	})
})

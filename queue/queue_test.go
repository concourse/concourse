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
	var builder *fakebuilder.Builder
	var queue *Queue

	var process ifrit.Process

	var job config.Job
	var resource config.Resource
	var version builds.Version

	BeforeEach(func() {
		gracePeriod = 100 * time.Millisecond
		builder = fakebuilder.New()

		job = config.Job{
			Name: "some-job",
		}

		resource = config.Resource{
			Name: "some-resource",
		}

		version = builds.Version{"ver": "1"}

		builder.BuildResult = builds.Build{
			ID: 42,
		}

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

			Ω(builder.Built()).Should(Equal([]fakebuilder.BuiltSpec{
				{
					Job: job,
				},
			}))
		})

		Context("and the queue is told to stop", func() {
			BeforeEach(func() {
				builder.WhenBuilding = func(config.Job, map[string]builds.Version) (builds.Build, error) {
					time.Sleep(time.Second)
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

			Ω(builder.Built()).Should(Equal([]fakebuilder.BuiltSpec{
				{
					Job: job,
					VersionOverrides: map[string]builds.Version{
						"some-resource": builds.Version{"ver": "1"},
					},
				},
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
				builder.BuildError = disaster
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

				Ω(builder.Built()).Should(Equal([]fakebuilder.BuiltSpec{
					{
						Job: job,
						VersionOverrides: map[string]builds.Version{
							"some-resource": builds.Version{"ver": "1"},
						},
					},
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

					Ω(builder.Built()).Should(Equal([]fakebuilder.BuiltSpec{
						{
							Job: job,
							VersionOverrides: map[string]builds.Version{
								"some-resource":   builds.Version{"ver": "1"},
								"second-resource": builds.Version{"ver": "2"},
							},
						},
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

					Ω(builder.Built()).Should(Equal([]fakebuilder.BuiltSpec{
						{
							Job: job,
							VersionOverrides: map[string]builds.Version{
								"some-resource": builds.Version{"ver": "1"},
							},
						},
						{
							Job: job,
							VersionOverrides: map[string]builds.Version{
								"second-resource": builds.Version{"ver": "2"},
							},
						},
					}))
				})
			})
		})
	})
})

package watchman_test

import (
	"time"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/queue/fakequeuer"
	"github.com/concourse/atc/redisrunner"
	"github.com/concourse/atc/resources/fakechecker"
	. "github.com/concourse/atc/watchman"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Watchman", func() {
	var redisRunner *redisrunner.Runner
	var redis db.DB

	var queuer *fakequeuer.FakeQueuer
	var watchman Watchman

	var job config.Job
	var resource config.Resource
	var checker *fakechecker.FakeChecker
	var eachVersion bool
	var interval time.Duration

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		redis = db.NewRedis(redisRunner.Pool())

		queuer = new(fakequeuer.FakeQueuer)

		watchman = NewWatchman(lagertest.NewTestLogger("watchman"), redis, queuer)

		job = config.Job{
			Name: "some-job",
			Inputs: []config.Input{
				{
					Resource: "some-resource",
				},
			},
		}

		resource = config.Resource{
			Name:   "some-resource",
			Type:   "git",
			Source: config.Source{"uri": "http://example.com"},
		}

		checker = new(fakechecker.FakeChecker)
		eachVersion = true
		interval = 100 * time.Millisecond
	})

	JustBeforeEach(func() {
		watchman.Watch(job, resource, checker, eachVersion, interval)
	})

	AfterEach(func() {
		watchman.Stop()
		redisRunner.Stop()
	})

	Describe("checking", func() {
		var times chan time.Time

		BeforeEach(func() {
			times = make(chan time.Time, 100)

			checker.CheckResourceStub = func(config.Resource, builds.Version) ([]builds.Version, error) {
				times <- time.Now()
				return nil, nil
			}
		})

		It("polls /checks on a specified interval", func() {
			var time1 time.Time
			var time2 time.Time

			Eventually(times).Should(Receive(&time1))
			Eventually(times).Should(Receive(&time2))

			Ω(time2.Sub(time1)).Should(BeNumerically("~", interval, interval/4))
		})

		Context("when there is no current version", func() {
			It("checks from nil", func() {
				Eventually(times).Should(Receive())

				resource, version := checker.CheckResourceArgsForCall(0)
				Ω(resource).Should(Equal(resource))
				Ω(version).Should(BeNil())
			})
		})

		Context("when there is a current version", func() {
			It("checks from it", func() {
				v1 := builds.Version{"version": "1"}
				v2 := builds.Version{"version": "2"}

				err := redis.SaveCurrentVersion(job.Name, resource.Name, v1)
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(times).Should(Receive())

				resource, version := checker.CheckResourceArgsForCall(0)
				Ω(resource).Should(Equal(resource))
				Ω(version).Should(Equal(v1))

				err = redis.SaveCurrentVersion(job.Name, resource.Name, v2)
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(times).Should(Receive())

				resource, version = checker.CheckResourceArgsForCall(1)
				Ω(resource).Should(Equal(resource))
				Ω(version).Should(Equal(v2))
			})
		})

		Context("when the check returns versions", func() {
			var checkedFrom chan builds.Version

			var nextVersions []builds.Version

			BeforeEach(func() {
				checkedFrom = make(chan builds.Version, 100)

				nextVersions = []builds.Version{
					{"version": "1"},
					{"version": "2"},
					{"version": "3"},
				}

				checkResults := map[int][]builds.Version{
					0: nextVersions,
				}

				check := 0
				checker.CheckResourceStub = func(checkedResource config.Resource, from builds.Version) ([]builds.Version, error) {
					defer GinkgoRecover()

					Ω(checkedResource).Should(Equal(resource))

					checkedFrom <- from
					result := checkResults[check]
					check++

					return result, nil
				}
			})

			It("enqueues a build for the job with the changed version", func() {
				Eventually(queuer.EnqueueCallCount).Should(Equal(3))

				job1, resource1, version1 := queuer.EnqueueArgsForCall(0)
				job2, resource2, version2 := queuer.EnqueueArgsForCall(1)
				job3, resource3, version3 := queuer.EnqueueArgsForCall(2)

				Ω(job1).Should(Equal(job))
				Ω(resource1).Should(Equal(resource))
				Ω(version1).Should(Equal(builds.Version{"version": "1"}))

				Ω(job2).Should(Equal(job))
				Ω(resource2).Should(Equal(resource))
				Ω(version2).Should(Equal(builds.Version{"version": "2"}))

				Ω(job3).Should(Equal(job))
				Ω(resource3).Should(Equal(resource))
				Ω(version3).Should(Equal(builds.Version{"version": "3"}))
			})

			Context("when configured to only build the latest versions", func() {
				BeforeEach(func() {
					eachVersion = false
				})

				It("only builds the latest version", func() {
					Eventually(queuer.EnqueueCallCount).Should(Equal(1))
					Consistently(queuer.EnqueueCallCount).Should(Equal(1))

					job1, resource1, version1 := queuer.EnqueueArgsForCall(0)
					Ω(job1).Should(Equal(job))
					Ω(resource1).Should(Equal(resource))
					Ω(version1).Should(Equal(builds.Version{"version": "3"}))
				})
			})
		})

		Context("and checking takes a while", func() {
			BeforeEach(func() {
				checked := false

				checker.CheckResourceStub = func(config.Resource, builds.Version) ([]builds.Version, error) {
					times <- time.Now()

					if checked {
						time.Sleep(interval)
					}

					checked = true

					return nil, nil
				}
			})

			It("does not count it towards the interval", func() {
				var time1 time.Time
				var time2 time.Time

				Eventually(times).Should(Receive(&time1))
				Eventually(times, 2).Should(Receive(&time2))

				Ω(time2.Sub(time1)).Should(BeNumerically("~", interval, interval/2))
			})
		})
	})
})

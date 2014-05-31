package watchman_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/winston-ci/winston/builder/fakebuilder"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/resources/fakechecker"
	. "github.com/winston-ci/winston/watchman"
)

var _ = Describe("Watchman", func() {
	var builder *fakebuilder.Builder
	var watchman Watchman

	var job config.Job
	var resource config.Resource
	var checker *fakechecker.FakeChecker
	var latestOnly bool
	var interval time.Duration

	BeforeEach(func() {
		builder = fakebuilder.New()

		watchman = NewWatchman(builder)

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

		checker = fakechecker.New()
		latestOnly = false
		interval = 100 * time.Millisecond
	})

	JustBeforeEach(func() {
		watchman.Watch(job, resource, nil, checker, latestOnly, interval)
	})

	AfterEach(func() {
		watchman.Stop()
	})

	Context("when watching", func() {
		var times chan time.Time

		BeforeEach(func() {
			times = make(chan time.Time, 100)

			checker.WhenCheckingResource = func(config.Resource, builds.Version) []builds.Version {
				times <- time.Now()
				return nil
			}
		})

		It("polls /checks on a specified interval", func() {
			var time1 time.Time
			var time2 time.Time

			Eventually(times).Should(Receive(&time1))
			Eventually(times).Should(Receive(&time2))

			Ω(time2.Sub(time1)).Should(BeNumerically("~", interval, interval/4))
		})

		Context("when the check returns sources", func() {
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
				checker.WhenCheckingResource = func(checkedResource config.Resource, from builds.Version) []builds.Version {
					defer GinkgoRecover()

					Ω(checkedResource).Should(Equal(resource))

					checkedFrom <- from
					result := checkResults[check]
					check++
					return result
				}
			})

			It("checks again from the previous source", func() {
				Eventually(checkedFrom).Should(Receive(BeNil()))
				Eventually(checkedFrom).Should(Receive(Equal(builds.Version{"version": "3"})))
			})

			It("builds the job with the changed source", func() {
				Eventually(builder.Built).Should(ContainElement(fakebuilder.BuiltSpec{
					Job: job,
					VersionOverrides: map[string]builds.Version{
						"some-resource": builds.Version{"version": "1"},
					},
				}))

				Eventually(builder.Built).Should(ContainElement(fakebuilder.BuiltSpec{
					Job: job,
					VersionOverrides: map[string]builds.Version{
						"some-resource": builds.Version{"version": "2"},
					},
				}))

				Eventually(builder.Built).Should(ContainElement(fakebuilder.BuiltSpec{
					Job: job,
					VersionOverrides: map[string]builds.Version{
						"some-resource": builds.Version{"version": "3"},
					},
				}))
			})

			Context("when configured to only build the latest sources", func() {
				BeforeEach(func() {
					latestOnly = true
				})

				It("only builds the latest source", func() {
					Eventually(builder.Built).Should(ContainElement(fakebuilder.BuiltSpec{
						Job: job,
						VersionOverrides: map[string]builds.Version{
							"some-resource": builds.Version{"version": "3"},
						},
					}))

					Consistently(builder.Built).Should(HaveLen(1))
				})

				It("checks again from the latest source", func() {
					Eventually(checkedFrom).Should(Receive(BeNil()))
					Eventually(checkedFrom).Should(Receive(Equal(builds.Version{"version": "3"})))
				})
			})
		})

		Context("and checking takes a while", func() {
			BeforeEach(func() {
				checked := false

				checker.WhenCheckingResource = func(config.Resource, builds.Version) []builds.Version {
					times <- time.Now()

					if checked {
						time.Sleep(interval)
					}

					checked = true

					return nil
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

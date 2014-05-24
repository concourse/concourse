package watchman_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/winston-ci/winston/builder/fakebuilder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/resources/fakechecker"
	. "github.com/winston-ci/winston/watchman"
)

var _ = Describe("Watchman", func() {
	var builder *fakebuilder.Builder
	var watchman Watchman

	var job config.Job
	var resource config.Resource
	var resources config.Resources
	var checker *fakechecker.FakeChecker
	var interval time.Duration

	var stop chan<- struct{}

	BeforeEach(func() {
		builder = fakebuilder.New()

		watchman = NewWatchman(builder)

		job = config.Job{
			Name:   "some-job",
			Inputs: config.InputMap{"some-input": nil},
		}

		resources = config.Resources{
			{
				Name:   "some-input",
				Type:   "git",
				Source: config.Source("123"),
			},
			{
				Name:   "some-other-input",
				Type:   "git",
				Source: config.Source("123"),
			},
		}

		resource = resources[0]

		checker = fakechecker.New()
		interval = 100 * time.Millisecond
	})

	JustBeforeEach(func() {
		stop = watchman.Watch(job, resource, resources, checker, interval)
	})

	AfterEach(func() {
		close(stop)
	})

	Context("when watching", func() {
		var times chan time.Time

		BeforeEach(func() {
			times = make(chan time.Time)

			checker.WhenCheckingResource = func(config.Resource) []config.Resource {
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
			var checkedFrom chan config.Resource

			var nextResources []config.Resource

			BeforeEach(func() {
				checkedFrom = make(chan config.Resource)

				nextResources = []config.Resource{
					{Name: "some-input", Type: "git", Source: config.Source("1")},
					{Name: "some-input", Type: "git", Source: config.Source("2")},
					{Name: "some-input", Type: "git", Source: config.Source("3")},
				}

				checkResults := map[int][]config.Resource{
					0: nextResources,
				}

				check := 0
				checker.WhenCheckingResource = func(from config.Resource) []config.Resource {
					checkedFrom <- from
					result := checkResults[check]
					check++
					return result
				}
			})

			It("checks again from the previous source", func() {
				Eventually(checkedFrom).Should(Receive(Equal(resource)))
				Eventually(checkedFrom).Should(Receive(Equal(nextResources[len(nextResources)-1])))
			})

			It("builds the job with the changed source", func() {
				Eventually(checkedFrom).Should(Receive())

				Eventually(builder.Built).Should(ContainElement(fakebuilder.BuiltSpec{
					Job: job,
					Resources: config.Resources{
						{
							Name:   "some-input",
							Type:   "git",
							Source: config.Source(`1`),
						},
						{
							Name:   "some-other-input",
							Type:   "git",
							Source: config.Source(`123`),
						},
					},
				}))

				Eventually(builder.Built).Should(ContainElement(fakebuilder.BuiltSpec{
					Job: job,
					Resources: config.Resources{
						{
							Name:   "some-input",
							Type:   "git",
							Source: config.Source(`2`),
						},
						{
							Name:   "some-other-input",
							Type:   "git",
							Source: config.Source(`123`),
						},
					},
				}))

				Eventually(builder.Built).Should(ContainElement(fakebuilder.BuiltSpec{
					Job: job,
					Resources: config.Resources{
						{
							Name:   "some-input",
							Type:   "git",
							Source: config.Source(`3`),
						},
						{
							Name:   "some-other-input",
							Type:   "git",
							Source: config.Source(`123`),
						},
					},
				}))
			})
		})

		Context("and checking takes a while", func() {
			BeforeEach(func() {
				checked := false

				checker.WhenCheckingResource = func(config.Resource) []config.Resource {
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

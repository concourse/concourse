package radar_test

import (
	"time"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"

	. "github.com/concourse/atc/radar"
	"github.com/concourse/atc/radar/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Radar", func() {
	var checker *fakes.FakeResourceChecker
	var tracker *fakes.FakeVersionDB
	var interval time.Duration

	var radar *Radar

	var resource config.Resource

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("radar")
		checker = new(fakes.FakeResourceChecker)
		tracker = new(fakes.FakeVersionDB)
		interval = 100 * time.Millisecond

		radar = NewRadar(logger, tracker, interval)

		resource = config.Resource{
			Name:   "some-resource",
			Type:   "git",
			Source: config.Source{"uri": "http://example.com"},
		}
	})

	JustBeforeEach(func() {
		radar.Scan(checker, resource)
	})

	AfterEach(func() {
		radar.Stop()
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

		It("checks on a specified interval", func() {
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
			BeforeEach(func() {
				tracker.GetLatestVersionedResourceReturns(builds.VersionedResource{
					Version: builds.Version{"version": "1"},
				}, nil)
			})

			It("checks from it", func() {
				Eventually(times).Should(Receive())

				resource, version := checker.CheckResourceArgsForCall(0)
				Ω(resource).Should(Equal(resource))
				Ω(version).Should(Equal(builds.Version{"version": "1"}))

				tracker.GetLatestVersionedResourceReturns(builds.VersionedResource{
					Version: builds.Version{"version": "2"},
				}, nil)

				Eventually(times).Should(Receive())

				resource, version = checker.CheckResourceArgsForCall(1)
				Ω(resource).Should(Equal(resource))
				Ω(version).Should(Equal(builds.Version{"version": "2"}))
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

			It("saves them all, in order", func() {
				Eventually(tracker.SaveVersionedResourceCallCount).Should(Equal(3))

				Ω(tracker.SaveVersionedResourceArgsForCall(0)).Should(Equal(builds.VersionedResource{
					Name:    "some-resource",
					Type:    "git",
					Source:  builds.Source{"uri": "http://example.com"},
					Version: builds.Version{"version": "1"},
				}))

				Ω(tracker.SaveVersionedResourceArgsForCall(1)).Should(Equal(builds.VersionedResource{
					Name:    "some-resource",
					Type:    "git",
					Source:  builds.Source{"uri": "http://example.com"},
					Version: builds.Version{"version": "2"},
				}))

				Ω(tracker.SaveVersionedResourceArgsForCall(2)).Should(Equal(builds.VersionedResource{
					Name:    "some-resource",
					Type:    "git",
					Source:  builds.Source{"uri": "http://example.com"},
					Version: builds.Version{"version": "3"},
				}))
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

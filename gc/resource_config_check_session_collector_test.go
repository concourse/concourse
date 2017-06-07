package gc_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCheckSessionCollector", func() {
	var collector gc.Collector
	var fakeResourceConfigCheckSessionLifecycle *dbfakes.FakeResourceConfigCheckSessionLifecycle

	BeforeEach(func() {
		fakeResourceConfigCheckSessionLifecycle = new(dbfakes.FakeResourceConfigCheckSessionLifecycle)
		fakeResourceConfigCheckSessionLifecycle.CleanExpiredResourceConfigCheckSessionsReturns(nil)

		logger := lagertest.NewTestLogger("resource-config-check-session-collector")
		collector = gc.NewResourceConfigCheckSessionCollector(logger, fakeResourceConfigCheckSessionLifecycle)
	})

	Describe("Run", func() {
		var runErr error

		JustBeforeEach(func() {
			runErr = collector.Run()
		})

		It("cleans up expired resource config check session", func() {
			Expect(fakeResourceConfigCheckSessionLifecycle.CleanExpiredResourceConfigCheckSessionsCallCount()).To(Equal(1))
		})

		Context("when cleaning up fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeResourceConfigCheckSessionLifecycle.CleanExpiredResourceConfigCheckSessionsReturns(disaster)
			})

			It("returns the error", func() {
				Expect(runErr).To(Equal(disaster))
			})
		})
	})
})

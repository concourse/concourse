package lidar_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/enginefakes"
	"github.com/concourse/concourse/atc/lidar"
)

type Checker interface {
	Run(context.Context) error
}

var _ = Describe("Checker", func() {
	var (
		err error

		fakeCheckFactory *dbfakes.FakeCheckFactory
		fakeEngine       *enginefakes.FakeEngine

		checker Checker
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeCheckFactory = new(dbfakes.FakeCheckFactory)
		fakeEngine = new(enginefakes.FakeEngine)

		logger = lagertest.NewTestLogger("test")
		checker = lidar.NewChecker(
			logger,
			fakeCheckFactory,
			fakeEngine,
		)
	})

	JustBeforeEach(func() {
		err = checker.Run(context.TODO())
	})

	Describe("Run", func() {

		Context("when retrieving checks fails", func() {
			BeforeEach(func() {
				fakeCheckFactory.StartedChecksReturns(nil, errors.New("nope"))
			})

			It("errors", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when retrieving checks succeeds", func() {
			var engineChecks []*enginefakes.FakeRunnable

			BeforeEach(func() {

				fakeCheckFactory.StartedChecksReturns([]db.Check{
					new(dbfakes.FakeCheck),
					new(dbfakes.FakeCheck),
					new(dbfakes.FakeCheck),
				}, nil)

				engineChecks = []*enginefakes.FakeRunnable{}
				fakeEngine.NewCheckStub = func(build db.Check) engine.Runnable {
					engineCheck := new(enginefakes.FakeRunnable)
					engineChecks = append(engineChecks, engineCheck)
					return engineCheck
				}
			})

			It("succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs all pendingg checks", func() {
				Eventually(engineChecks[0].RunCallCount).Should(Equal(1))
				Eventually(engineChecks[1].RunCallCount).Should(Equal(1))
				Eventually(engineChecks[2].RunCallCount).Should(Equal(1))
			})
		})
	})
})

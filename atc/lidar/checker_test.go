package lidar_test

import (
	"context"
	"errors"
	"time"

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

			BeforeEach(func() {
				fakeCheck1 := new(dbfakes.FakeCheck)
				fakeCheck1.IDReturns(1)
				fakeCheck2 := new(dbfakes.FakeCheck)
				fakeCheck2.IDReturns(2)
				fakeCheck3 := new(dbfakes.FakeCheck)
				fakeCheck3.IDReturns(3)

				fakeCheckFactory.StartedChecksReturns([]db.Check{
					fakeCheck1,
					fakeCheck2,
					fakeCheck3,
				}, nil)

				fakeEngine.NewCheckStub = func(build db.Check) engine.Runnable {
					time.Sleep(time.Second)
					return new(enginefakes.FakeRunnable)
				}
			})

			It("succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs all pending checks", func() {
				Eventually(fakeEngine.NewCheckCallCount).Should(Equal(3))
			})
		})

		Context("when a check is already running", func() {

			BeforeEach(func() {
				fakeCheck := new(dbfakes.FakeCheck)
				fakeCheck.IDReturns(1)

				fakeEngine.NewCheckStub = func(build db.Check) engine.Runnable {
					time.Sleep(time.Second)
					return new(enginefakes.FakeRunnable)
				}

				fakeCheckFactory.StartedChecksReturns([]db.Check{
					fakeCheck,
					fakeCheck,
				}, nil)
			})

			It("succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs only one pending check", func() {
				Eventually(fakeEngine.NewCheckCallCount).Should(Equal(1))
			})
		})
	})
})

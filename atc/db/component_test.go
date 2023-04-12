package db_test

import (
	"time"

	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Component", func() {
	var (
		err       error
		found     bool
		component db.Component
	)

	BeforeEach(func() {
		_, err = dbConn.Exec("INSERT INTO components (name, interval) VALUES ('scheduler', '5s') ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval")
		Expect(err).NotTo(HaveOccurred())

		component, found, err = componentFactory.Find("scheduler")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
	})

	JustBeforeEach(func() {
		reloaded, err := component.Reload()
		Expect(reloaded).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("IntervalElapsed", func() {
		var elapsed bool

		BeforeEach(func() {
			err = component.UpdateLastRan()
			Expect(err).NotTo(HaveOccurred())

			// Sync fake clock's now to last run
			fakeCompClock.Increment(component.LastRan().Sub(fakeCompClock.Now()))
		})

		JustBeforeEach(func() {
			elapsed = component.IntervalElapsed()
		})

		Context("when no goroutine threshold set", func() {
			Context("when there is no drift", func() {
				BeforeEach(func() {
					fakeRander.IntReturns(int(time.Second))
					// Make current time a little earlier than next run
					fakeCompClock.Increment(component.Interval() - 50*time.Millisecond)
				})

				Context("when the interval is not reached", func() {
					It("returns false", func() {
						Expect(elapsed).To(BeFalse())
					})
				})

				Context("when the interval is reached", func() {
					BeforeEach(func() {
						fakeCompClock.Increment(1 * time.Second)
					})

					It("returns true", func() {
						Expect(elapsed).To(BeTrue())
					})
				})
			})

			Context("when there is some drift", func() {
				BeforeEach(func() {
					fakeCompClock.Increment(component.Interval() - 50*time.Millisecond)
				})

				Context("drift -100 millisecond", func() {
					BeforeEach(func() {
						fakeRander.IntReturns(int(900 * time.Millisecond))
					})
					It("returns true", func() {
						Expect(elapsed).To(BeTrue())
					})
				})

				Context("drift -10 millisecond", func() {
					BeforeEach(func() {
						fakeRander.IntReturns(int(990 * time.Millisecond))
					})
					It("returns false", func() {
						Expect(elapsed).To(BeFalse())

						fakeCompClock.Increment(41 * time.Millisecond)
						elapsed = component.IntervalElapsed()
						Expect(elapsed).To(BeTrue())
					})
				})

				Context("drift 10 millisecond", func() {
					BeforeEach(func() {
						fakeRander.IntReturns(int(1010 * time.Millisecond))
					})
					It("returns false", func() {
						Expect(elapsed).To(BeFalse())

						fakeCompClock.Increment(61 * time.Millisecond)
						elapsed = component.IntervalElapsed()
						Expect(elapsed).To(BeTrue())
					})
				})
			})
		})

		Context("when goroutine threshold set", func() {
			BeforeEach(func() {
				componentFactory = db.NewComponentFactory(dbConn, 50000, fakeRander, fakeCompClock, fakeGoroutineCounter)

				component, found, err = componentFactory.Find("scheduler")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = component.UpdateLastRan()
				Expect(err).NotTo(HaveOccurred())

				fakeCompClock.Increment(component.LastRan().Sub(fakeCompClock.Now()))
			})

			It("should not call rand once", func() {
				Expect(fakeRander.IntCallCount()).To(Equal(0))
			})

			Context("when there are 0 goroutine", func() {
				BeforeEach(func() {
					fakeGoroutineCounter.NumGoroutineReturns(0)
					// Make current time reach to next run
					fakeCompClock.Increment(component.Interval() - time.Second + 10*time.Millisecond)
				})

				It("should drift -1 second", func() {
					Expect(elapsed).To(BeTrue())
				})
			})

			Context("when there are 50000 goroutine", func() {
				BeforeEach(func() {
					fakeGoroutineCounter.NumGoroutineReturns(50000)
					// Make current time reach to next run
					fakeCompClock.Increment(component.Interval() + 10*time.Millisecond)
				})

				It("should drift 0 second", func() {
					Expect(elapsed).To(BeTrue())
				})
			})

			Context("when there are 100000 goroutine", func() {
				BeforeEach(func() {
					fakeGoroutineCounter.NumGoroutineReturns(100000)
					// Make current time reach to next run
					fakeCompClock.Increment(component.Interval() + 10*time.Millisecond)
				})

				It("should drift 1 second", func() {
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeTrue())
				})
			})

			Context("when there are 100000 goroutine", func() {
				BeforeEach(func() {
					fakeGoroutineCounter.NumGoroutineReturns(100000)
					// Make current time reach to next run
					fakeCompClock.Increment(component.Interval() + 10*time.Millisecond)
				})

				It("should drift 1 second", func() {
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeTrue())
				})
			})

			Context("when there are 150000 goroutine", func() {
				BeforeEach(func() {
					fakeGoroutineCounter.NumGoroutineReturns(150000)
					// Make current time reach to next run
					fakeCompClock.Increment(component.Interval() + 10*time.Millisecond)
				})

				It("should drift 2 second", func() {
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeTrue())
				})
			})

			Context("when there are 200000 goroutine", func() {
				BeforeEach(func() {
					fakeGoroutineCounter.NumGoroutineReturns(200000)
					// Make current time reach to next run
					fakeCompClock.Increment(component.Interval() + 10*time.Millisecond)
				})

				It("should drift 3 second", func() {
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeTrue())
				})
			})

			Context("when there are 1 million goroutine", func() {
				BeforeEach(func() {
					fakeGoroutineCounter.NumGoroutineReturns(1000000)
					// Make current time reach to next run
					fakeCompClock.Increment(component.Interval() + 10*time.Millisecond)
				})

				It("should drift 5 second", func() {
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeFalse())

					fakeCompClock.Increment(1 * time.Second)
					elapsed = component.IntervalElapsed()
					Expect(elapsed).To(BeTrue())
				})
			})
		})
	})

	Describe("UpdateLastRan", func() {
		BeforeEach(func() {
			err = component.UpdateLastRan()
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates component last ran time", func() {
			lastRan := component.LastRan()
			Expect(lastRan).To(BeTemporally("~", time.Now(), time.Second))
		})
	})
})

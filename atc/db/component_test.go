package db_test

import (
	"time"

	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Component", func() {
	var (
		err       error
		found     bool
		component db.Component
	)

	BeforeEach(func() {
		_, err = dbConn.Exec("INSERT INTO components (name, interval) VALUES ('scheduler', '100ms') ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval")
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

		JustBeforeEach(func() {
			elapsed = component.IntervalElapsed()
		})

		Context("when the interval is not reached", func() {
			BeforeEach(func() {
				err = component.UpdateLastRan()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns false", func() {
				Expect(elapsed).To(BeFalse())
			})
		})

		Context("when the interval is reached", func() {
			BeforeEach(func() {
				err = component.UpdateLastRan()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(1 * time.Second)
			})

			It("returns true", func() {
				Expect(elapsed).To(BeTrue())
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

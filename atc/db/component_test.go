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

				// TODO maybe a fake clock so no need to sleep here?
				time.Sleep(11 * time.Second)
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
			Expect(lastRan).To(BeTemporally("~", time.Now()))
		})
	})
})

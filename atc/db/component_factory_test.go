package db_test

import (
	"time"

	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ComponentFactory", func() {

	var (
		err          error
		found        bool
		component    db.Component
		expectedName = "scheduler"
	)

	BeforeEach(func() {
		_, err = dbConn.Exec("INSERT INTO components (name, interval) VALUES ('scheduler', '100ms') ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Find", func() {
		BeforeEach(func() {
			component, found, err = componentFactory.Find(expectedName)
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the db component", func() {
			Expect(component.Name()).To(Equal(expectedName))
			Expect(component.Interval()).To(Equal("100ms"))
			Expect(component.Paused()).To(Equal(false))
		})
	})

	Describe("UpdateIntervals", func() {
		expectedInterval := 42 * time.Second

		BeforeEach(func() {
			err = componentFactory.UpdateIntervals(
				map[string]time.Duration{
					expectedName: expectedInterval,
				})
			Expect(err).NotTo(HaveOccurred())

			component, found, err = componentFactory.Find(expectedName)
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates component interval", func() {
			interval := component.Interval()
			Expect(interval).To(Equal(expectedInterval.String()))
		})
	})
})

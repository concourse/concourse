package db_test

import (
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ComponentFactory", func() {
	Describe("Find", func() {
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

		JustBeforeEach(func() {
			component, found, err = componentFactory.Find(expectedName)
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the db component", func() {
			Expect(component.Name()).To(Equal(expectedName))
			Expect(component.Interval()).To(Equal(100 * time.Millisecond))
			Expect(component.Paused()).To(Equal(false))
		})
	})

	Describe("CreateOrUpdate", func() {
		It("updates component interval", func() {
			interval := 1 * time.Second
			componentName := "some-component"

			createdComponent, err := componentFactory.CreateOrUpdate(atc.Component{
				Name:     componentName,
				Interval: interval,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(createdComponent.ID()).ToNot(BeZero())
			Expect(createdComponent.Name()).To(Equal(componentName))
			Expect(createdComponent.Interval()).To(Equal(interval))

			updatedComponent, err := componentFactory.CreateOrUpdate(atc.Component{
				Name:     componentName,
				Interval: interval + 1,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedComponent.ID()).To(Equal(createdComponent.ID()))
			Expect(updatedComponent.Name()).To(Equal(componentName))
			Expect(updatedComponent.Interval()).To(Equal(interval + 1))

			foundComponent, found, err := componentFactory.Find(componentName)
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
			Expect(foundComponent.ID()).To(Equal(updatedComponent.ID()))
			Expect(foundComponent.Name()).To(Equal(componentName))
			Expect(foundComponent.Interval()).To(Equal(interval + 1))
		})
	})
})

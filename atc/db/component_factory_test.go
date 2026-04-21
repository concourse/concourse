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

	Describe("All", func() {
		BeforeEach(func() {
			_, err := dbConn.Exec("INSERT INTO components (name, interval) VALUES ('comp-a', '100ms') ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval")
			Expect(err).NotTo(HaveOccurred())
			_, err = dbConn.Exec("INSERT INTO components (name, interval) VALUES ('comp-b', '200ms') ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns all components", func() {
			components, err := componentFactory.All()
			Expect(err).NotTo(HaveOccurred())

			names := []string{}
			for _, c := range components {
				names = append(names, c.Name())
			}
			Expect(names).To(ContainElements("comp-a", "comp-b"))
		})
	})

	Describe("PauseAll", func() {
		BeforeEach(func() {
			_, err := dbConn.Exec("INSERT INTO components (name, interval) VALUES ('comp-a', '100ms') ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval")
			Expect(err).NotTo(HaveOccurred())
			_, err = dbConn.Exec("INSERT INTO components (name, interval) VALUES ('comp-b', '200ms') ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval")
			Expect(err).NotTo(HaveOccurred())
		})

		It("pauses all components", func() {
			err := componentFactory.PauseAll()
			Expect(err).NotTo(HaveOccurred())

			compA, found, err := componentFactory.Find("comp-a")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(compA.Paused()).To(BeTrue())

			compB, found, err := componentFactory.Find("comp-b")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(compB.Paused()).To(BeTrue())
		})
	})

	Describe("UnpauseAll", func() {
		BeforeEach(func() {
			_, err := dbConn.Exec("INSERT INTO components (name, interval, paused) VALUES ('comp-a', '100ms', true) ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval, paused = EXCLUDED.paused")
			Expect(err).NotTo(HaveOccurred())
			_, err = dbConn.Exec("INSERT INTO components (name, interval, paused) VALUES ('comp-b', '200ms', true) ON CONFLICT (name) DO UPDATE SET interval = EXCLUDED.interval, paused = EXCLUDED.paused")
			Expect(err).NotTo(HaveOccurred())
		})

		It("unpauses all components", func() {
			err := componentFactory.UnpauseAll()
			Expect(err).NotTo(HaveOccurred())

			compA, found, err := componentFactory.Find("comp-a")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(compA.Paused()).To(BeFalse())

			compB, found, err := componentFactory.Find("comp-b")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(compB.Paused()).To(BeFalse())
		})
	})
})

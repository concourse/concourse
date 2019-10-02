package db_test

import (
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ComponentFactory", func() {

	var (
		err       error
		found     bool
		component db.Component
	)

	Describe("Find", func() {
		BeforeEach(func() {
			component, found, err = componentFactory.Find("scheduler")
		})

		It("succeeds", func() {
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the db component", func() {
			Expect(component.Name()).To(Equal("scheduler"))
			Expect(component.Interval()).To(Equal("10s"))
			Expect(component.Paused()).To(Equal(false))
		})
	})
})

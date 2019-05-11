package db_test

import (
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Check", func() {
	var check db.Check
	var err error

	BeforeEach(func() {
		check, err = checkFactory.CreateCheck(1, db.CheckTypeResource)
		Expect(err).NotTo(HaveOccurred())
	})

	FDescribe("ResourceConfigScope", func() {
		Context("when looking up resource config scope succeeds", func() {
			It("returns the resource", func() {
				resource, err := check.ResourceConfigScope()

				Expect(err).NotTo(HaveOccurred())
				Expect(resource.ID()).To(Equal(1))
				// Expect(resource.Name()).To(Equal("some-resource"))
				// Expect(resource.Type()).To(Equal("some-base-resource-type"))
				// Expect(resource.Source()).To(Equal(atc.Source{"some": "source"}))
			})
		})
	})
})

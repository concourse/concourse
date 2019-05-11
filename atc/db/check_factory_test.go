package db_test

import (
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckFactory", func() {
	Describe("Checks", func() {
		Context("when looking up the resource check succeeds", func() {
			var check db.Check
			var err error

			BeforeEach(func() {
				check, err = checkFactory.CreateCheck(1, db.CheckTypeResource)
				Expect(err).NotTo(HaveOccurred())
			})

			FIt("returns the resource check", func() {
				checks, err := checkFactory.Checks()

				Expect(err).NotTo(HaveOccurred())
				Expect(checks).To(HaveLen(1))
				Expect(checks[0]).To(Equal(check))
			})
		})
	})
})

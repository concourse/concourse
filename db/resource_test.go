package db_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db"
)

var _ = Describe("Resource", func() {
	Describe("FailingToCheck", func() {
		It("returns true if there is a check error", func() {
			resource := db.SavedResource{
				CheckError: errors.New("nope"),
			}

			Expect(resource.FailingToCheck()).To(BeTrue())
		})

		It("returns false if there is no check error", func() {
			resource := db.SavedResource{
				CheckError: nil,
			}
			Expect(resource.FailingToCheck()).To(BeFalse())
		})
	})
})

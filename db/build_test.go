package db_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db"
)

var _ = Describe("Build", func() {
	Describe("IsRunning", func() {
		It("returns true if the build is pending", func() {
			build := db.Build{
				Status: db.StatusPending,
			}
			Expect(build.Abortable()).To(BeTrue())
		})

		It("returns true if the build is started", func() {
			build := db.Build{
				Status: db.StatusStarted,
			}
			Expect(build.Abortable()).To(BeTrue())
		})

		It("returns false if in any other state", func() {
			states := []db.Status{
				db.StatusAborted,
				db.StatusErrored,
				db.StatusFailed,
				db.StatusSucceeded,
			}

			for _, state := range states {
				build := db.Build{
					Status: state,
				}
				Expect(build.Abortable()).To(BeFalse())
			}
		})
	})

	Describe("Abortable", func() {
		It("returns true if the build is pending", func() {
			build := db.Build{
				Status: db.StatusPending,
			}
			Expect(build.Abortable()).To(BeTrue())
		})

		It("returns true if the build is started", func() {
			build := db.Build{
				Status: db.StatusStarted,
			}
			Expect(build.Abortable()).To(BeTrue())
		})

		It("returns false if in any other state", func() {
			states := []db.Status{
				db.StatusAborted,
				db.StatusErrored,
				db.StatusFailed,
				db.StatusSucceeded,
			}

			for _, state := range states {
				build := db.Build{
					Status: state,
				}
				Expect(build.Abortable()).To(BeFalse())
			}
		})
	})
})

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

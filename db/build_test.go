package db_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db"
)

var _ = Describe("Build", func() {
	Describe("OneOff", func() {
		It("returns true if there is no JobName", func() {
			build := db.Build{
				JobName: "",
			}
			Ω(build.OneOff()).Should(BeTrue())
		})

		It("returns false if there is a JobName", func() {
			build := db.Build{
				JobName: "something",
			}
			Ω(build.OneOff()).Should(BeFalse())
		})
	})

	Describe("Abortable", func() {
		It("returns true if the build is pending", func() {
			build := db.Build{
				Status: db.StatusPending,
			}
			Ω(build.Abortable()).Should(BeTrue())
		})

		It("returns true if the build is started", func() {
			build := db.Build{
				Status: db.StatusStarted,
			}
			Ω(build.Abortable()).Should(BeTrue())
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
				Ω(build.Abortable()).Should(BeFalse())
			}
		})
	})
})

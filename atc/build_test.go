package atc_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Build", func() {
	Describe("OneOff", func() {
		It("returns true if there is no JobName", func() {
			build := atc.Build{
				JobName: "",
			}
			Expect(build.OneOff()).To(BeTrue())
		})

		It("returns false if there is a JobName", func() {
			build := atc.Build{
				JobName: "something",
			}
			Expect(build.OneOff()).To(BeFalse())
		})
	})

	Describe("IsRunning", func() {
		It("returns true if the build is pending", func() {
			build := atc.Build{
				Status: string(atc.StatusPending),
			}
			Expect(build.Abortable()).To(BeTrue())
		})

		It("returns true if the build is started", func() {
			build := atc.Build{
				Status: string(atc.StatusStarted),
			}
			Expect(build.Abortable()).To(BeTrue())
		})

		It("returns false if in any other state", func() {
			states := []atc.BuildStatus{
				atc.StatusAborted,
				atc.StatusErrored,
				atc.StatusFailed,
				atc.StatusSucceeded,
			}

			for _, state := range states {
				build := atc.Build{
					Status: string(state),
				}
				Expect(build.Abortable()).To(BeFalse())
			}
		})
	})

	Describe("Abortable", func() {
		It("returns true if the build is pending", func() {
			build := atc.Build{
				Status: string(atc.StatusPending),
			}
			Expect(build.Abortable()).To(BeTrue())
		})

		It("returns true if the build is started", func() {
			build := atc.Build{
				Status: string(atc.StatusStarted),
			}
			Expect(build.Abortable()).To(BeTrue())
		})

		It("returns false if in any other state", func() {
			states := []db.BuildStatus{
				db.BuildStatusAborted,
				db.BuildStatusErrored,
				db.BuildStatusFailed,
				db.BuildStatusSucceeded,
			}

			for _, state := range states {
				build := atc.Build{
					Status: string(state),
				}
				Expect(build.Abortable()).To(BeFalse())
			}
		})
	})
})

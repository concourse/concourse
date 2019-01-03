package exec_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunState", func() {
	var state exec.RunState

	BeforeEach(func() {
		state = exec.NewRunState()
	})

	Describe("Result", func() {
		var (
			id atc.PlanID
			to interface{}

			ok bool
		)

		BeforeEach(func() {
			id = "some-id"

			someVal := 42
			to = &someVal
		})

		JustBeforeEach(func() {
			ok = state.Result(id, to)
		})

		Context("when no result is present", func() {
			BeforeEach(func() {
				// do nothing
			})

			It("does not mutate the var", func() {
				v := 42
				Expect(to).To(Equal(&v))
			})

			It("returns false", func() {
				Expect(ok).To(BeFalse())
			})
		})

		Context("when a result under a different id is present", func() {
			BeforeEach(func() {
				state.StoreResult(id+"-other", 42)
			})

			It("does not mutate the var", func() {
				v := 42
				Expect(to).To(Equal(&v))
			})

			It("returns false", func() {
				Expect(ok).To(BeFalse())
			})
		})

		Context("when a result under the given id is present", func() {
			BeforeEach(func() {
				state.StoreResult(id, 123)
			})

			It("mutates the var", func() {
				v := 123
				Expect(to).To(Equal(&v))
			})

			It("returns true", func() {
				Expect(ok).To(BeTrue())
			})

			Context("but with a different type", func() {
				BeforeEach(func() {
					state.StoreResult(id, "one hundred and twenty-three")
				})

				It("does not mutate the var", func() {
					v := 42
					Expect(to).To(Equal(&v))
				})

				It("returns false", func() {
					Expect(ok).To(BeFalse())
				})
			})
		})
	})
})

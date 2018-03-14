package exec_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"runtime"

	"github.com/concourse/atc"
	"github.com/concourse/atc/exec"
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

	Describe("User Input", func() {
		It("can be passed around asynchronously", func() {
			buf := ioutil.NopCloser(bytes.NewBufferString("some-payload"))

			for i := 0; i < 1000; i++ {
				go state.SendUserInput("some-plan-id", buf)

				state.ReadUserInput("some-plan-id", func(rc io.ReadCloser) error {
					Expect(rc).To(Equal(buf))
					return nil
				})
			}
		})

		It("blocks the sender until the reader is finished", func() {
			buf := ioutil.NopCloser(bytes.NewBufferString("some-payload"))

			var done bool
			go func() {
				defer GinkgoRecover()
				state.SendUserInput("some-plan-id", buf)
				Expect(done).To(BeTrue())
			}()

			runtime.Gosched()

			err := state.ReadUserInput("some-plan-id", func(rc io.ReadCloser) error {
				Expect(rc).To(Equal(buf))
				done = true
				return nil
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("bubbles up the handler error", func() {
			buf := ioutil.NopCloser(bytes.NewBufferString("some-payload"))

			go state.SendUserInput("some-plan-id", buf)

			disaster := errors.New("nope")
			err := state.ReadUserInput("some-plan-id", func(rc io.ReadCloser) error {
				return disaster
			})
			Expect(err).To(Equal(disaster))
		})
	})

	Describe("Plan Output", func() {
		It("can be passed around asynchronously", func() {
			out := new(bytes.Buffer)

			for i := 0; i < 1000; i++ {
				go state.ReadPlanOutput("some-plan-id", out)

				state.SendPlanOutput("some-plan-id", func(w io.Writer) error {
					Expect(w).To(Equal(out))
					return nil
				})
			}
		})

		It("blocks the sender until the reader is finished", func() {
			buf := new(bytes.Buffer)

			var done bool
			go func() {
				defer GinkgoRecover()
				state.ReadPlanOutput("some-plan-id", buf)
				Expect(done).To(BeTrue())
			}()

			runtime.Gosched()

			err := state.SendPlanOutput("some-plan-id", func(w io.Writer) error {
				Expect(w).To(Equal(buf))
				done = true
				return nil
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("bubbles up the handler error", func() {
			buf := new(bytes.Buffer)

			go state.ReadPlanOutput("some-plan-id", buf)

			disaster := errors.New("nope")
			err := state.SendPlanOutput("some-plan-id", func(w io.Writer) error {
				return disaster
			})
			Expect(err).To(Equal(disaster))
		})
	})
})

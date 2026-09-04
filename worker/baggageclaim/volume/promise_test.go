package volume

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume Promise", func() {
	var (
		promise    Promise
		testErr    = errors.New("some-error")
		testVolume = Volume{
			Handle:     "some-handle",
			Path:       "some-path",
			Properties: make(Properties),
		}
	)

	BeforeEach(func() {
		promise = NewPromise()
	})

	Context("newly created", func() {
		It("is pending", func() {
			Expect(promise.IsPending()).To(BeTrue())
		})

		It("can not return a value yet", func() {
			_, _, err := promise.GetValue()

			Expect(err).To(Equal(ErrPromiseStillPending))
		})
	})

	Context("when fulfilled", func() {
		It("is not pending", func() {
			promise.Fulfill(testVolume)

			Expect(promise.IsPending()).To(BeFalse())
		})

		It("returns a non-empty volume in value", func() {
			promise.Fulfill(testVolume)

			val, _, _ := promise.GetValue()

			Expect(val).To(Equal(testVolume))
		})

		It("returns a nil error in value", func() {
			promise.Fulfill(testVolume)

			_, val, _ := promise.GetValue()

			Expect(val).To(BeNil())
		})

		It("can return a value", func() {
			promise.Fulfill(testVolume)

			_, _, err := promise.GetValue()

			Expect(err).To(BeNil())
		})

		Context("when not pending", func() {
			Context("when canceled", func() {
				It("returns ErrPromiseCanceled", func() {
					promise.Reject(ErrPromiseCanceled)

					err := promise.Fulfill(testVolume)

					Expect(err).To(Equal(ErrPromiseCanceled))
				})
			})

			Context("when fulfilled", func() {
				It("returns ErrPromiseNotPending", func() {
					promise.Fulfill(testVolume)

					err := promise.Fulfill(testVolume)

					Expect(err).To(Equal(ErrPromiseNotPending))
				})
			})

			Context("when rejected", func() {
				It("returns ErrPromiseNotPending", func() {
					promise.Reject(testErr)

					err := promise.Fulfill(testVolume)

					Expect(err).To(Equal(ErrPromiseNotPending))
				})
			})
		})
	})

	Context("when rejected", func() {
		It("is not pending", func() {
			promise.Reject(testErr)

			Expect(promise.IsPending()).To(BeFalse())
		})

		It("returns an empty volume in value", func() {
			promise.Reject(testErr)

			val, _, _ := promise.GetValue()

			Expect(val).To(Equal(Volume{}))
		})

		It("returns a non-nil error in value", func() {
			promise.Reject(testErr)

			_, val, _ := promise.GetValue()

			Expect(val).To(Equal(testErr))
		})

		It("can return a value", func() {
			promise.Reject(testErr)

			_, _, err := promise.GetValue()

			Expect(err).To(BeNil())
		})

		Context("when rejecting again", func() {
			Context("when canceled", func() {
				It("returns ErrPromiseNotPending", func() {
					promise.Reject(ErrPromiseCanceled)

					err := promise.Reject(testErr)

					Expect(err).To(Equal(ErrPromiseNotPending))
				})
			})

			Context("when not canceled", func() {
				It("returns ErrPromiseNotPending", func() {
					promise.Reject(testErr)

					err := promise.Reject(testErr)

					Expect(err).To(Equal(ErrPromiseNotPending))
				})
			})
		})
	})
})

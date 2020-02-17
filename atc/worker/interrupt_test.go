package worker_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/concourse/atc/worker"
)

var _ = Describe("Interrupt", func() {
	Describe("storing an interrupt timeout", func() {
		Context("with a nil context", func() {
			It("should store the value", func() {
				newCtx := WithInterruptTimeout(nil, 1*time.Minute)
				Expect(newCtx).ToNot(BeNil())
				val, ok := InterruptTimeoutFromContext(newCtx)
				Expect(ok).To(BeTrue())
				Expect(val).To(Equal(1 * time.Minute))
			})
		})

		Context("with any other context", func() {
			It("should store the value", func() {
				newCtx := WithInterruptTimeout(context.TODO(), 1*time.Minute)
				Expect(newCtx).ToNot(BeNil())
				val, ok := InterruptTimeoutFromContext(newCtx)
				Expect(ok).To(BeTrue())
				Expect(val).To(Equal(1 * time.Minute))
			})
		})
	})
})

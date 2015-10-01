package worker_test

import (
	"fmt"
	"time"

	. "github.com/concourse/atc/worker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExponentialRetryPolicy", func() {
	var (
		attempts uint

		policy RetryPolicy

		delay        time.Duration
		keepRetrying bool
	)

	BeforeEach(func() {
		policy = ExponentialRetryPolicy{
			Timeout: 5 * time.Minute,
		}

		attempts = 0
	})

	JustBeforeEach(func() {
		delay, keepRetrying = policy.DelayFor(attempts)
	})

	type row struct {
		attempts     uint
		delay        time.Duration
		keepRetrying bool
	}

	for _, row := range []row{
		{0, 0 * time.Second, true},
		{1, 1 * time.Second, true},
		{2, 2 * time.Second, true},
		{3, 4 * time.Second, true},
		{4, 8 * time.Second, true},
		{5, 16 * time.Second, true},
		{6, 16 * time.Second, true},
		{7, 16 * time.Second, true},
		{8, 16 * time.Second, true},
		{9, 16 * time.Second, true},
		{20, 16 * time.Second, true},
		{21, 0, false},
		{22, 0, false},
	} {
		row := row

		Context(fmt.Sprintf("after %d failed attempts", row.attempts), func() {
			BeforeEach(func() {
				attempts = row.attempts
			})

			It(fmt.Sprintf("returns a %s delay", row.delay), func() {
				Expect(delay).To(Equal(row.delay))
			})

			if row.keepRetrying {
				It("keeps retrying", func() {
					Expect(keepRetrying).To(BeTrue())
				})
			} else {
				It("gives up", func() {
					Expect(keepRetrying).To(BeFalse())
				})
			}
		})
	}
})

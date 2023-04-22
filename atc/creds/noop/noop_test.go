package noop_test

import (
	"time"

	. "github.com/concourse/concourse/atc/creds/noop"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Noop", func() {
	var noop Noop

	Describe("Get", func() {
		var val interface{}
		var expiration *time.Time
		var found bool
		var getErr error

		JustBeforeEach(func() {
			val, expiration, found, getErr = noop.Get("foo")
		})

		It("never locates the variable", func() {
			Expect(val).To(BeNil())
			Expect(expiration).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(getErr).ToNot(HaveOccurred())
		})
	})
})

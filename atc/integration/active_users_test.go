package integration_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("ATC Integration Test", func() {
	It("active users are tracked", func() {
		client := login(atcURL, "test", "test")
		twoMonthsAgo := time.Now().AddDate(0, -2, 0)
		users, _ := client.ListActiveUsersSince(twoMonthsAgo)
		Expect(users).To(ConsistOf(MatchFields(IgnoreExtras,
			Fields{"Username": Equal("test")},
		)))
	})
})

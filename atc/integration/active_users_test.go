package integration_test

import (
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/atccmd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("ATC Integration Test", func() {
	It("active users are tracked", func() {
		client := login(atcURL, "test", "test")
		twoMonthsAgo := time.Now().AddDate(0, -2, 0)
		Eventually(func() []atc.User {
			users, _ := client.ListActiveUsersSince(twoMonthsAgo)
			return users
		}, atccmd.BatcherInterval).Should(
			ConsistOf(
				MatchFields(IgnoreExtras,
					Fields{"Username": Equal("test")},
				),
			),
		)
	})
})

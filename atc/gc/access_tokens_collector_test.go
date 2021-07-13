package gc_test

import (
	"context"

	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/gc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/square/go-jose.v2/jwt"
)

var _ = Describe("AccessTokensCollector", func() {
	var collector GcCollector
	var fakeLifecycle *dbfakes.FakeAccessTokenLifecycle

	BeforeEach(func() {
		fakeLifecycle = new(dbfakes.FakeAccessTokenLifecycle)

		collector = gc.NewAccessTokensCollector(fakeLifecycle, jwt.DefaultLeeway)
	})

	Describe("Run", func() {
		It("tells the access token lifecycle to remove expired access tokens", func() {
			err := collector.Run(context.TODO())
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLifecycle.RemoveExpiredAccessTokensCallCount()).To(Equal(1))
			leeway := fakeLifecycle.RemoveExpiredAccessTokensArgsForCall(0)
			Expect(leeway).To(Equal(jwt.DefaultLeeway))
		})
	})
})

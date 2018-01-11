package bitbucket_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"errors"
	"github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/bitbucket/bitbucketfakes"
	"github.com/concourse/skymarshal/verifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
)

var _ = Describe("RepositoryVerifier", func() {
	var (
		repositories []bitbucket.RepositoryConfig
		fakeClient   *bitbucketfakes.FakeClient

		verifier verifier.Verifier
	)

	BeforeEach(func() {
		repositories = []bitbucket.RepositoryConfig{
			{OwnerName: "some-team-two", RepositoryName: "some-repository"},
			{OwnerName: "some-team", RepositoryName: "some-repository"},
		}
		fakeClient = new(bitbucketfakes.FakeClient)

		verifier = bitbucket.NewRepositoryVerifier(repositories, fakeClient)
	})

	Describe("Verify", func() {
		var (
			httpClient *http.Client

			verified  bool
			verifyErr error
		)

		BeforeEach(func() {
			httpClient = &http.Client{}
		})

		JustBeforeEach(func() {
			verified, verifyErr = verifier.Verify(lagertest.NewTestLogger("test"), httpClient)
		})

		Context("when the client finds the desired repository", func() {
			BeforeEach(func() {
				fakeClient.RepositoryReturnsOnCall(0, false, nil)
				fakeClient.RepositoryReturnsOnCall(1, true, nil)
			})

			It("succeeds", func() {
				Expect(verifyErr).ToNot(HaveOccurred())
			})

			It("returns true", func() {
				Expect(verified).To(BeTrue())
			})
		})

		Context("when the client doesn't find the desired repository", func() {
			BeforeEach(func() {
				fakeClient.RepositoryReturns(false, nil)
			})

			It("succeeds", func() {
				Expect(verifyErr).ToNot(HaveOccurred())
			})

			It("returns false", func() {
				Expect(verified).To(BeFalse())
			})
		})

		Context("when the client fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeClient.RepositoryReturns(false, disaster)
			})

			It("returns the error", func() {
				Expect(verifyErr).To(Equal(disaster))
			})
		})
	})
})

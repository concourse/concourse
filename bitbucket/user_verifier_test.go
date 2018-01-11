package bitbucket_test

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/bitbucket/bitbucketfakes"
	"github.com/concourse/skymarshal/verifier"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UserVerifier", func() {
	var (
		users      []string
		fakeClient *bitbucketfakes.FakeClient

		verifier verifier.Verifier
	)

	BeforeEach(func() {
		users = []string{
			"some-user",
			"some-user-two",
		}
		fakeClient = new(bitbucketfakes.FakeClient)

		verifier = NewUserVerifier(users, fakeClient)
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

		Context("when the client returns the current user", func() {
			Context("when the user is permitted", func() {
				BeforeEach(func() {
					fakeClient.CurrentUserReturns("some-user", nil)
				})

				It("succeeds", func() {
					Expect(verifyErr).ToNot(HaveOccurred())
				})

				It("returns true", func() {
					Expect(verified).To(BeTrue())
				})
			})

			Context("when the user is not permitted", func() {
				BeforeEach(func() {
					fakeClient.CurrentUserReturns("some-other-user", nil)
				})

				It("succeeds", func() {
					Expect(verifyErr).ToNot(HaveOccurred())
				})

				It("returns false", func() {
					Expect(verified).To(BeFalse())
				})
			})
		})

		Context("when the client fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeClient.CurrentUserReturns("", disaster)
			})

			It("returns the error", func() {
				Expect(verifyErr).To(Equal(disaster))
			})
		})
	})
})

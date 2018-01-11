package github_test

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/skymarshal/github"
	"github.com/concourse/skymarshal/github/githubfakes"
	"github.com/concourse/skymarshal/verifier"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("OrganizationVerifier", func() {
	var (
		organizations []string
		fakeClient    *githubfakes.FakeClient

		verifier verifier.Verifier
	)

	BeforeEach(func() {
		organizations = []string{"some-organization", "another-organization"}
		fakeClient = new(githubfakes.FakeClient)

		verifier = NewOrganizationVerifier(organizations, fakeClient)
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

		Context("when the client yields organizations", func() {
			Context("including one of the desired organizations", func() {
				BeforeEach(func() {
					fakeClient.OrganizationsReturns([]string{organizations[0], "bogus-organization"}, nil)
				})

				It("succeeds", func() {
					Expect(verifyErr).ToNot(HaveOccurred())
				})

				It("returns true", func() {
					Expect(verified).To(BeTrue())
				})
			})

			Context("not including the desired organizations", func() {
				BeforeEach(func() {
					fakeClient.OrganizationsReturns([]string{"bogus-organization"}, nil)
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
				fakeClient.OrganizationsReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(verifyErr).To(Equal(disaster))
			})
		})
	})
})

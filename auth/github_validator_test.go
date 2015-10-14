package auth_test

import (
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/github/fakes"
)

var _ = Describe("GitHubOrganizationValidator", func() {
	var fakeGitHubClient *fakes.FakeClient
	var validator auth.GitHubOrganizationValidator

	BeforeEach(func() {
		fakeGitHubClient = new(fakes.FakeClient)
		validator = auth.GitHubOrganizationValidator{
			Organization: "testOrg",
			Client:       fakeGitHubClient,
		}
	})

	Describe("IsAuthenticated", func() {
		var request *http.Request
		var authenticated bool

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", "http://example.com/something", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			authenticated = validator.IsAuthenticated(request)
		})

		Context("when the request has an Authorization header", func() {
			Context("with a valid token", func() {
				BeforeEach(func() {
					request.Header.Add("Authorization", "Token abcd")
				})

				It("strips out the Token keyword before calling to github", func() {
					Expect(fakeGitHubClient.GetOrganizationsCallCount()).To(Equal(1))
					Expect(fakeGitHubClient.GetOrganizationsArgsForCall(0)).To(Equal("abcd"))
				})

				Context("when authentication fails", func() {
					Context("because the github client errors", func() {
						BeforeEach(func() {
							fakeGitHubClient.GetOrganizationsReturns(nil, errors.New("disaster"))
						})

						It("returns false", func() {
							Expect(authenticated).To(BeFalse())
						})
					})

					Context("because the given token is not in the org", func() {
						BeforeEach(func() {
							fakeGitHubClient.GetOrganizationsReturns([]string{"nope"}, nil)
						})

						It("returns false", func() {
							Expect(authenticated).To(BeFalse())
						})
					})
				})

				Context("with the correct credentials", func() {
					BeforeEach(func() {
						fakeGitHubClient.GetOrganizationsReturns([]string{"testOrg"}, nil)
					})

					It("returns true", func() {
						Expect(authenticated).To(BeTrue())
					})
				})
			})

			Context("with a bogus token", func() {
				BeforeEach(func() {
					request.Header.Add("Authorization", "abcd")
				})

				It("returns false", func() {
					Expect(authenticated).To(BeFalse())
				})

				It("does not call to github", func() {
					Expect(fakeGitHubClient.GetOrganizationsCallCount()).To(Equal(0))
				})
			})
		})
	})
})

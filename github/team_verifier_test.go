package github_test

import (
	"errors"
	"net/http"

	"github.com/concourse/atc/auth"
	. "github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/auth/github/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TeamVerifier", func() {
	var (
		teams      []string
		fakeClient *fakes.FakeClient

		verifier auth.Verifier
	)

	BeforeEach(func() {
		teams = []string{
			"some-org/some-team",
			"some-org/some-team-two",
			"some-awesome-org/all",
		}
		fakeClient = new(fakes.FakeClient)

		verifier = NewTeamVerifier(teams, fakeClient)
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
			verified, verifyErr = verifier.Verify(httpClient)
		})

		Context("when the client yields teams", func() {
			Context("including the desired team", func() {
				BeforeEach(func() {
					fakeClient.TeamsReturns(
						OrganizationTeams{
							"some-other-org": {"some-other-team"},
							"some-org":       {"some-team"},
						},
						nil,
					)
				})

				It("succeeds", func() {
					Expect(verifyErr).ToNot(HaveOccurred())
				})

				It("returns true", func() {
					Expect(verified).To(BeTrue())
				})
			})

			Context("including the desired org if the team is set to all", func() {
				BeforeEach(func() {
					fakeClient.TeamsReturns(
						OrganizationTeams{
							"some-other-org":   {"some-other-team"},
							"some-awesome-org": {"some-team-that-does-not-matter"},
						},
						nil,
					)
				})

				It("succeeds", func() {
					Expect(verifyErr).ToNot(HaveOccurred())
				})

				It("returns true", func() {
					Expect(verified).To(BeTrue())
				})
			})

			Context("not including the team", func() {
				BeforeEach(func() {
					fakeClient.TeamsReturns(
						OrganizationTeams{
							"some-other-org": {"some-team"},
						},
						nil,
					)
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
				fakeClient.TeamsReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(verifyErr).To(Equal(disaster))
			})
		})
	})
})

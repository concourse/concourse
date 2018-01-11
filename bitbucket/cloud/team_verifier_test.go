package cloud_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"errors"
	"github.com/concourse/skymarshal/bitbucket/bitbucketfakes"
	"github.com/concourse/skymarshal/bitbucket/cloud"
	"github.com/concourse/skymarshal/verifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
)

var _ = Describe("TeamVerifier", func() {
	var (
		teams      []string
		role       cloud.Role
		fakeClient *bitbucketfakes.FakeClient

		verifier verifier.Verifier
	)

	BeforeEach(func() {
		teams = []string{
			"some-team",
			"some-team-two",
		}
		role = cloud.RoleContributor
		fakeClient = new(bitbucketfakes.FakeClient)

		verifier = cloud.NewTeamVerifier(teams, role, fakeClient)
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

		Context("when the client yields teams", func() {
			Context("including the desired team", func() {
				BeforeEach(func() {
					fakeClient.TeamsReturns(
						[]string{
							"some-other-team",
							"some-team",
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

			Context("not including the desired team", func() {
				BeforeEach(func() {
					fakeClient.TeamsReturns(
						[]string{
							"some-other-team",
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

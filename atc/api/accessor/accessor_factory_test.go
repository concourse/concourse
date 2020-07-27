package accessor_test

import (
	"errors"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
)

var _ = Describe("AccessorFactory", func() {
	var (
		systemClaimKey    string
		systemClaimValues []string

		fakeTokenVerifier *accessorfakes.FakeTokenVerifier
		fakeTeamFetcher   *accessorfakes.FakeTeamFetcher
		dummyRequest      *http.Request

		role  string
	)

	BeforeEach(func() {
		systemClaimKey = "sub"
		systemClaimValues = []string{"some-sub"}

		fakeTokenVerifier = new(accessorfakes.FakeTokenVerifier)
		fakeTeamFetcher = new(accessorfakes.FakeTeamFetcher)
		dummyRequest, _ = http.NewRequest("GET", "/", nil)

		role = "viewer"
	})

	Describe("Create", func() {

		var (
			access accessor.Access
			err    error
		)

		JustBeforeEach(func() {
			factory := accessor.NewAccessFactory(fakeTokenVerifier, fakeTeamFetcher, systemClaimKey, systemClaimValues)
			access, err = factory.Create(dummyRequest, role)
		})

		Context("when the token is valid", func() {
			BeforeEach(func() {
				fakeTokenVerifier.VerifyReturns(map[string]interface{}{
					"federated_claims": map[string]interface{}{
						"connector_id": "github",
						"user_name":    "user1",
					},
				}, nil)
				teamWithUsers := func(name string, authenticated bool) db.Team {
					t := new(dbfakes.FakeTeam)
					t.NameReturns(name)
					if authenticated {
						t.AuthReturns(atc.TeamAuth{"viewer": map[string][]string{
							"users": {"github:user1"},
						}})
					}
					return t
				}
				fakeTeamFetcher.GetTeamsReturns([]db.Team{
					teamWithUsers("t1", true),
					teamWithUsers("t2", false),
					teamWithUsers("t3", true),
				}, nil)
			})

			It("returns an accessor with the correct teams", func() {
				Expect(access.TeamNames()).To(ConsistOf("t1", "t3"))
			})
		})

		Context("when the team fetcher returns an error", func() {
			BeforeEach(func() {
				fakeTeamFetcher.GetTeamsReturns(nil, errors.New("nope"))
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the verifier returns a NoToken error", func() {
			BeforeEach(func() {
				fakeTokenVerifier.VerifyReturns(nil, accessor.ErrVerificationNoToken)
			})

			It("the accessor has no token", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(access.HasToken()).To(BeFalse())
			})
		})

		Context("when the verifier returns some other error", func() {
			BeforeEach(func() {
				fakeTokenVerifier.VerifyReturns(nil, accessor.ErrVerificationTokenExpired)
			})

			It("the accessor is unauthenticated", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(access.IsAuthenticated()).To(BeFalse())
			})
		})
	})
})

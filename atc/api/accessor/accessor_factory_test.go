package accessor_test

import (
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
)

var _ = Describe("AccessorFactory", func() {
	var (
		err    error
		access accessor.Access

		accessorFactory accessor.AccessFactory
		req             *http.Request

		fakeVerifier    *accessorfakes.FakeVerifier
		fakeTeamFactory *dbfakes.FakeTeamFactory
	)

	BeforeEach(func() {
		req, err = http.NewRequest("GET", "localhost:8080", nil)
		Expect(err).NotTo(HaveOccurred())

		fakeVerifier = new(accessorfakes.FakeVerifier)
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)

		accessorFactory = accessor.NewAccessFactory(
			fakeVerifier,
			fakeTeamFactory,
			"sub",
			[]string{"some-sub"},
			map[string]string{"some-role": "some-action"},
		)
	})

	Describe("Create", func() {

		JustBeforeEach(func() {
			access, err = accessorFactory.Create(req, "some-role")
		})

		Context("when the verifier returns an NoToken error", func() {
			BeforeEach(func() {
				fakeVerifier.VerifyReturns(nil, accessor.ErrVerificationNoToken)
			})

			It("does not query for teams", func() {
				Expect(fakeTeamFactory.GetTeamsCallCount()).To(Equal(0))
			})

			It("creates an accessor", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(access.HasToken()).To(BeFalse())
			})
		})

		Context("when the verifier returns any other error", func() {
			BeforeEach(func() {
				fakeVerifier.VerifyReturns(nil, errors.New("some error"))
			})

			It("does not query for teams", func() {
				Expect(fakeTeamFactory.GetTeamsCallCount()).To(Equal(0))
			})

			It("creates an accessor", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(access.HasToken()).To(BeTrue())
				Expect(access.IsAuthenticated()).To(BeFalse())
			})
		})

		Context("when the verifier returns claims", func() {
			var claims map[string]interface{}

			BeforeEach(func() {
				claims = map[string]interface{}{
					"sub": "some-sub",
					"aud": "some-aud",
				}

				fakeVerifier.VerifyReturns(claims, nil)
			})

			It("queries for teams", func() {
				Expect(fakeTeamFactory.GetTeamsCallCount()).To(Equal(1))
			})

			Context("when the team factory returns an error", func() {
				BeforeEach(func() {
					fakeTeamFactory.GetTeamsReturns(nil, errors.New("nope"))
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when the team factory returns teams", func() {
				var teams []db.Team

				BeforeEach(func() {
					fakeTeam1 := new(dbfakes.FakeTeam)
					fakeTeam2 := new(dbfakes.FakeTeam)
					fakeTeam3 := new(dbfakes.FakeTeam)

					teams = []db.Team{fakeTeam1, fakeTeam2, fakeTeam3}

					fakeTeamFactory.GetTeamsReturns(teams, nil)
				})

				It("creates an accessor", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(access.HasToken()).To(BeTrue())
					Expect(access.IsAuthenticated()).To(BeTrue())
				})
			})
		})
	})
})

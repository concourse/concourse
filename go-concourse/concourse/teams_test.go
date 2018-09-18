package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Teams", func() {
	Describe("CreateOrUpdate", func() {
		var expectedURL = "/api/v1/teams/team venture"
		var expectedTeam, desiredTeam atc.Team

		BeforeEach(func() {
			desiredTeam = atc.Team{
				Name: "team venture",
			}
			expectedTeam = atc.Team{
				ID:   1,
				Name: "team venture",
			}

			team = client.Team("team venture")
		})

		Context("when passed a properly constructed team", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSONRepresenting(desiredTeam),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedTeam),
					),
				)
			})

			It("returns back the team", func() {
				team, _, _, err := team.CreateOrUpdate(desiredTeam)
				Expect(err).NotTo(HaveOccurred())
				Expect(team).To(Equal(expectedTeam))
			})
		})

		Context("when passed a team that doesn't exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSONRepresenting(desiredTeam),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedTeam),
					),
				)
			})

			It("returns back true for created, and false for updated", func() {
				_, found, updated, err := team.CreateOrUpdate(desiredTeam)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(updated).To(BeFalse())
			})
		})

		Context("when passed a team that exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSONRepresenting(desiredTeam),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedTeam),
					),
				)
			})

			It("returns back false for created, and true for updated", func() {
				_, found, updated, err := team.CreateOrUpdate(desiredTeam)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(updated).To(BeTrue())
			})
		})
	})

	Describe("Destroy", func() {
		var (
			expectedURL string
			err         error
		)

		BeforeEach(func() {
			expectedURL = "/api/v1/teams/enron"
			team = client.Team("not-super-important")
		})

		JustBeforeEach(func() {
			err = team.DestroyTeam("enron")
		})

		Context("when passed a team that you can't delete", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL),
						ghttp.RespondWith(http.StatusForbidden, nil),
					),
				)
			})

			It("returns back true for created, and false for updated", func() {
				Expect(err).To(Equal(concourse.ErrDestroyRefused))
			})
		})

		Context("when the server deletes the team", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
				)
			})

			It("returns back false for created, and true for updated", func() {
				Expect(err).To(BeNil())
			})
		})

		Context("when the server blows up", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, nil),
					),
				)
			})

			It("returns back false for created, and true for updated", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).NotTo(Equal(concourse.ErrDestroyRefused))
			})
		})
	})

	Describe("ListTeams", func() {
		var expectedTeams []atc.Team

		BeforeEach(func() {
			expectedURL := "/api/v1/teams"

			expectedTeams = []atc.Team{
				{
					ID:   1,
					Name: "main",
				},
				{
					ID:   2,
					Name: "a-team",
				},
				{
					ID:   3,
					Name: "b-team",
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedTeams),
				),
			)
		})

		It("returns all of the teams", func() {
			teams, err := client.ListTeams()
			Expect(err).NotTo(HaveOccurred())
			Expect(teams).To(Equal(expectedTeams))
		})
	})
})

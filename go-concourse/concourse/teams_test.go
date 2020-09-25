package concourse_test

import (
	"errors"
	"github.com/concourse/concourse/atc/api/teamserver"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/concourse/go-concourse/concourse/internal"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Teams", func() {
	Describe("FindTeam", func() {
		teamName := "myTeam"
		expectedURL := "/api/v1/teams/myTeam"
		expectedAuth := atc.TeamAuth{
			"owner": map[string][]string{
				"groups": {}, "users": {"local:username"},
			},
		}

		Context("when the team is found", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
							ID:   1,
							Name: teamName,
							Auth: expectedAuth,
						}),
					),
				)
			})

			It("returns the requested team", func() {
				team, err := client.FindTeam(teamName)
				Expect(err).NotTo(HaveOccurred())
				Expect(team.Name()).To(Equal(teamName))
				Expect(team.Auth()).To(Equal(expectedAuth))
			})
		})

		Context("when the team is not found", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := client.FindTeam(teamName)
				Expect(err).To(Equal(errors.New("team 'myTeam' does not exist")))

			})
		})

		Context("when not belonging to the team", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and error", func() {
				_, err := client.FindTeam(teamName)
				Expect(err).To(Equal(errors.New("team 'myTeam' does not exist")))
			})
		})

		Context("when an unhandled HTTP status code is returned", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, "server issue"),
					),
				)
			})
			It("returns an UnexpectedResponseError", func() {
				_, err := client.FindTeam(teamName)
				Expect(err).To(Equal(internal.UnexpectedResponseError{
					StatusCode: http.StatusInternalServerError,
					Status:     "500 Internal Server Error",
					Body:       "server issue",
				}))
			})
		})
	})
	Describe("CreateOrUpdate", func() {
		var expectedURL = "/api/v1/teams/team venture"
		var expectedTeam, desiredTeam atc.Team
		var expectedResponse teamserver.SetTeamResponse

		BeforeEach(func() {
			desiredTeam = atc.Team{
				Name: "team venture",
			}
			expectedTeam = atc.Team{
				ID:   1,
				Name: "team venture",
			}
			expectedResponse.Team = expectedTeam

			team = client.Team("team venture")
		})

		Context("when passed a properly constructed team", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSONRepresenting(desiredTeam),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedResponse),
					),
				)
			})

			It("returns back the team", func() {
				team, _, _, _, err := team.CreateOrUpdate(desiredTeam)
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
						ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedResponse),
					),
				)
			})

			It("returns back true for created, and false for updated", func() {
				_, found, updated, _, err := team.CreateOrUpdate(desiredTeam)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(updated).To(BeFalse())
			})
		})

		Context("when passed a team that exists", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSONRepresenting(desiredTeam),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedResponse),
					),
				)
			})

			It("returns back false for created, and true for updated", func() {
				_, found, updated, _, err := team.CreateOrUpdate(desiredTeam)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(updated).To(BeTrue())
			})

			Context("when the team has an invalid identifier", func() {
				BeforeEach(func() {
					expectedResponse.Warnings = []atc.ConfigWarning{
						{Type: "invalid_identifier",
							Message: "team: 'new venture' is not a valid identifier",
						},
					}
				})

				It("returns back false for created, and true for updated", func() {
					_, _, _, warnings, err := team.CreateOrUpdate(desiredTeam)
					Expect(err).NotTo(HaveOccurred())
					Expect(warnings).To(HaveLen(1))
					Expect(warnings[0].Message).To(ContainSubstring("team: 'new venture' is not a valid identifier"))
				})
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

package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Teams", func() {
	Describe("SetTeam", func() {
		var expectedURL = "/api/v1/teams/team venture"
		var expectedTeam, desiredTeam atc.Team

		BeforeEach(func() {
			desiredTeam = atc.Team{
				BasicAuth: atc.BasicAuth{
					BasicAuthUsername: "Brock Samson",
					BasicAuthPassword: "John. Bonham. Rocks.",
				},
			}
			expectedTeam = atc.Team{
				ID:   1,
				Name: "team venture",
			}
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
				team, _, _, err := client.SetTeam("team venture", desiredTeam)
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
				_, found, updated, err := client.SetTeam("team venture", desiredTeam)
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
				_, found, updated, err := client.SetTeam("team venture", desiredTeam)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(updated).To(BeTrue())
			})
		})
	})
})

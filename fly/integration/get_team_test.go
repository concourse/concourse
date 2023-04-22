package integration_test

import (
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("Fly CLI", func() {
	Describe("get-team", func() {
		var team atc.Team

		BeforeEach(func() {
			team = atc.Team{
				ID:   1,
				Name: "myTeam",
				Auth: atc.TeamAuth{
					"owner": map[string][]string{
						"groups": {}, "users": {"local:username"},
					},
				},
			}
		})

		Context("when not specifying a team name", func() {
			It("fails and says you should give a team name", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "get-team")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("n", "team-name") + "' was not specified"))
			})
		})

		Context("when specifying a team name", func() {
			var path string
			BeforeEach(func() {
				var err error
				path, err = atc.Routes.CreatePathForRoute(atc.GetTeam, rata.Params{"team_name": "myTeam"})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("and team is not found", func() {
				JustBeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", path),
							ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
						),
					)
				})

				It("should print team not found error", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "get-team", "-n", "myTeam")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				})
			})

			Context("when atc returns team config", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", path),
							ghttp.RespondWithJSONEncoded(200, team),
						),
					)
				})

				It("prints the team config to stdout", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "get-team", "-n", "myTeam")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out).To(PrintTable(ui.Table{
						Headers: ui.TableRow{
							{Contents: "name/role", Color: color.New(color.Bold)},
							{Contents: "users", Color: color.New(color.Bold)},
							{Contents: "groups", Color: color.New(color.Bold)},
						},
						Data: []ui.TableRow{
							{{Contents: "myTeam/owner"}, {Contents: "local:username"}, {Contents: "none"}},
						},
					}))
				})

				It("produces structured JSON output if requested", func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "get-team", "-n", "myTeam", "--json")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					Expect(sess.Out.Contents()).To(MatchJSON(`{"id": 1, "name": "myTeam", "auth": {"owner": {"groups": [], "users": ["local:username"] }}}`))
				})
			})
		})
	})
})

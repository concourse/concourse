package integration_test

import (
	"fmt"
	"io"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("destroy-team", func() {
		var (
			stdin io.Writer
			args  []string
			sess  *gexec.Session
		)

		BeforeEach(func() {
			stdin = nil
			args = []string{}
		})

		JustBeforeEach(func() {
			var err error

			flyCmd := exec.Command(flyPath, append([]string{"-t", targetName, "destroy-team"}, args...)...)
			stdin, err = flyCmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the team name is not specified", func() {
			It("asks the user for the team name", func() {
				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("n", "team-name") + "' was not specified"))
			})
		})

		Context("when the team name is secified", func() {
			BeforeEach(func() {
				args = append(args, "-n", "some-team")
			})

			typesName := func(name string) {
				fmt.Fprintf(stdin, "%s\n", name)
			}

			It("reminds the user this is a destructive operation", func() {
				Eventually(sess).Should(gbytes.Say("!!! this will remove all data for team `some-team`"))
			})

			It("asks the user to type in the team name again", func() {
				Eventually(sess).Should(gbytes.Say(`are you sure\? please type the team name to continue:`))
			})

			Context("when the user types in the name again succesfully", func() {
				JustBeforeEach(func() {
					typesName("some-team")
				})

				Context("when the api call returns 204 (succesful)", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("DELETE", "/api/v1/teams/some-team"),
								ghttp.RespondWith(204, ""),
							),
						)
					})

					It("tells the user the team has been destoryed", func() {
						Eventually(sess).Should(gbytes.Say(`some-team has been destroyed`))
					})
				})

				Context("when the api call returns 403 Forbidden", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("DELETE", "/api/v1/teams/some-team"),
								ghttp.RespondWith(403, ""),
							),
						)
					})
					It("tells the user that they are not allowed to do this", func() {
						Eventually(sess).Should(gbytes.Say(`only admin can delete teams`))
					})
				})
				Context("when the api call returns 500 Internal Server Error", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("DELETE", "/api/v1/teams/some-team"),
								ghttp.RespondWith(500, ""),
							),
						)
					})

					It("tells the user that there was a 500 Internal Server Error", func() {
						Eventually(sess).Should(gbytes.Say(`delete failed due to server error`))
					})
				})
			})

			Context("when the user fails to type in the team name again succesfully", func() {
				JustBeforeEach(func() {
					typesName("not-the-correct-team-name")
				})

				It("asks them to try again", func() {
					Eventually(sess).Should(gexec.Exit(1))
					Expect(sess.Err).To(gbytes.Say(`you typed in the team name incorrectly, bailing out`))
				})
			})
		})
	})
})

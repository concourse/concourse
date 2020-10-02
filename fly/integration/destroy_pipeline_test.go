package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("destroy-pipeline", func() {
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

			flyCmd := exec.Command(flyPath, append([]string{"-t", targetName, "destroy-pipeline"}, args...)...)
			stdin, err = flyCmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when a pipeline name is not specified", func() {
			It("asks the user to specify a pipeline name", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "destroy-pipeline")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("p", "pipeline") + "' was not specified"))
			})
		})

		Context("when the pipeline flag is invalid", func() {
			It("fails and print invalid flag error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "destroy-pipeline", "-p", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: invalid argument for flag `" + osFlag("p", "pipeline")))
			})
		})

		Context("when a pipeline name is specified", func() {
			BeforeEach(func() {
				args = append(args, "-p", "some-pipeline/branch:master")
			})

			yes := func() {
				Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
				fmt.Fprintf(stdin, "y\n")
			}

			no := func() {
				Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
				fmt.Fprintf(stdin, "n\n")
			}

			queryParams := "instance_vars=%7B%22branch%22%3A%22master%22%7D"

			It("warns that it's about to do bad things", func() {
				Eventually(sess).Should(gbytes.Say("!!! this will remove all data for pipeline `some-pipeline/branch:master`"))
			})

			It("bails out if the user says no", func() {
				no()
				Eventually(sess).Should(gbytes.Say(`bailing out`))
				Eventually(sess).Should(gexec.Exit(0))
			})

			Context("when the pipeline exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/some-pipeline", queryParams),
							ghttp.RespondWith(204, ""),
						),
					)
				})

				It("succeeds if the user says yes", func() {
					yes()
					Eventually(sess).Should(gbytes.Say("`some-pipeline/branch:master` deleted"))
					Eventually(sess).Should(gexec.Exit(0))
				})

				Context("when run noninteractively", func() {
					BeforeEach(func() {
						args = append(args, "-n")
					})

					It("destroys the pipeline without confirming", func() {
						Eventually(sess).Should(gbytes.Say("`some-pipeline/branch:master` deleted"))
						Eventually(sess).Should(gexec.Exit(0))
					})
				})
			})

			Context("and the pipeline does not exist", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/some-pipeline", queryParams),
							ghttp.RespondWith(404, ""),
						),
					)
				})

				It("writes that it did not exist and exits successfully", func() {
					yes()
					Eventually(sess).Should(gbytes.Say("`some-pipeline/branch:master` does not exist"))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("and the api returns an unexpected status code", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/some-pipeline", queryParams),
							ghttp.RespondWith(402, ""),
						),
					)
				})

				It("writes an error message to stderr", func() {
					yes()
					Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("with a team specified", func() {
				BeforeEach(func() {
					args = append(args, "--team", "team-two")

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/api/v1/teams/team-two"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
								Name: "team-two",
							}),
						),
					)
				})

				It("warns that it's about to do bad things", func() {
					Eventually(sess).Should(gbytes.Say("!!! this will remove all data for pipeline `some-pipeline/branch:master`"))
				})

				It("bails out if the user says no", func() {
					no()
					Eventually(sess).Should(gbytes.Say(`bailing out`))
					Eventually(sess).Should(gexec.Exit(0))
				})

				Context("when the pipeline exists", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("DELETE", "/api/v1/teams/team-two/pipelines/some-pipeline", queryParams),
								ghttp.RespondWith(204, ""),
							),
						)
					})

					It("succeeds if the user says yes", func() {
						yes()
						Eventually(sess).Should(gbytes.Say("`some-pipeline/branch:master` deleted"))
						Eventually(sess).Should(gexec.Exit(0))
					})
				})

				Context("and the pipeline does not exist", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("DELETE", "/api/v1/teams/team-two/pipelines/some-pipeline", queryParams),
								ghttp.RespondWith(404, ""),
							),
						)
					})

					It("writes that it did not exist and exits successfully", func() {
						yes()
						Eventually(sess).Should(gbytes.Say("`some-pipeline/branch:master` does not exist"))
						Eventually(sess).Should(gexec.Exit(0))
					})
				})
			})
		})
	})
})

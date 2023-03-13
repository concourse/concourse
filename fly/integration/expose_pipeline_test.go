package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("Fly CLI", func() {
	Describe("expose-pipeline", func() {
		Context("when the pipeline name is specified", func() {
			var (
				path        string
				queryParams string
				err         error
			)
			BeforeEach(func() {
				path, err = atc.Routes.CreatePathForRoute(atc.ExposePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
				Expect(err).NotTo(HaveOccurred())

				queryParams = "vars.branch=%22master%22"
			})

			Context("when the pipeline exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path, queryParams),
							ghttp.RespondWith(http.StatusOK, nil),
						),
					)
				})

				It("exposes the pipeline", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "expose-pipeline", "-p", "awesome-pipeline/branch:master")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`exposed 'awesome-pipeline/branch:master'`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(0))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})

			Context("when the pipeline doesn't exist", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", path),
							ghttp.RespondWith(http.StatusNotFound, nil),
						),
					)
				})

				It("prints helpful message", func() {
					Expect(func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "expose-pipeline", "-p", "awesome-pipeline")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess.Err).Should(gbytes.Say(`pipeline 'awesome-pipeline' not found`))

						<-sess.Exited
						Expect(sess.ExitCode()).To(Equal(1))
					}).To(Change(func() int {
						return len(atcServer.ReceivedRequests())
					}).By(2))
				})
			})
		})

		Context("when the pipline name is not specified", func() {
			It("errors", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "expose-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					<-sess.Exited
					Expect(sess.ExitCode()).To(Equal(1))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(0))
			})
		})

		Context("when the pipeline flag is invalid", func() {
			It("fails and print invalid flag error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "expose-pipeline", "-p", "forbidden/pipelinename")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: invalid argument for flag `" + osFlag("p", "pipeline")))
			})
		})

		Context("user is NOT targeting the same team the pipeline belongs to", func() {
			team := "diff-team"
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/%s", team)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
							Name: team,
						}),
					),
				)
			})

			Context("when the pipeline name is specified", func() {
				var (
					path        string
					queryParams string
					err         error
				)
				BeforeEach(func() {
					path, err = atc.Routes.CreatePathForRoute(atc.ExposePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": team})
					Expect(err).NotTo(HaveOccurred())

					queryParams = "vars.branch=%22master%22"
				})

				Context("when the pipeline exists", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", path, queryParams),
								ghttp.RespondWith(http.StatusOK, nil),
							),
						)
					})

					It("exposes the pipeline", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "expose-pipeline", "-p", "awesome-pipeline/branch:master", "--team", team)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say(`exposed 'awesome-pipeline/branch:master'`))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(3))
					})
				})

				Context("when the pipeline doesn't exist", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", path),
								ghttp.RespondWith(http.StatusNotFound, nil),
							),
						)
					})

					It("prints helpful message", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "expose-pipeline", "-p", "awesome-pipeline", "--team", team)

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess.Err).Should(gbytes.Say(`pipeline 'awesome-pipeline' not found`))

							<-sess.Exited
							Expect(sess.ExitCode()).To(Equal(1))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(3))
					})
				})
			})
		})

	})
})

package integration_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/v8/atc"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("unpause-pipeline", func() {
	DescribeTable("rejects invalid instance var arguments without API requests", func(arguments []string, message string) {
		requestCount := len(atcServer.ReceivedRequests())
		flyCmd := exec.Command(flyPath, append([]string{"-t", targetName, "unpause-pipeline"}, arguments...)...)

		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(sess).Should(gexec.Exit(1))
		Expect(sess.Err).To(gbytes.Say(message))
		Expect(atcServer.ReceivedRequests()).To(HaveLen(requestCount))
	},
		Entry("without a pipeline", []string{"-i", "branch=master"}, "--instance-var requires --pipeline"),
		Entry("with --all", []string{"--all", "-i", "branch=master"}, "--instance-var requires --pipeline"),
		Entry("with inline instance vars", []string{"-p", "awesome-pipeline/branch:master", "-i", "env=prod"}, "instance vars specified both in pipeline name and via --instance-var"),
		Entry("with a malformed pair", []string{"-p", "awesome-pipeline", "-i", "branch"}, "must be name=value"),
	)

	Context("when the pipeline name is specified", func() {
		var (
			mainPath    string
			otherPath   string
			queryParams string
			err         error
		)
		BeforeEach(func() {
			mainPath, err = atc.Routes.CreatePathForRoute(atc.UnpausePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
			Expect(err).NotTo(HaveOccurred())

			otherPath, err = atc.Routes.CreatePathForRoute(atc.UnpausePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "other-team"})
			Expect(err).NotTo(HaveOccurred())

			queryParams = "vars.branch=%22master%22"
		})

		DescribeTable("unpauses a pipeline instance using flags", func(arguments []string, query string) {
			atcServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", mainPath, query),
				ghttp.RespondWith(http.StatusOK, nil),
			))
			flyCmd := exec.Command(flyPath, append([]string{"-t", targetName, "unpause-pipeline", "-p", "awesome-pipeline"}, arguments...)...)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say(`unpaused 'awesome-pipeline/branch:master`))
		},
			Entry("short flag", []string{"-i", "branch=master"}, "vars.branch=%22master%22"),
			Entry("long flag", []string{"--instance-var", "branch=master"}, "vars.branch=%22master%22"),
			Entry("repeated YAML values", []string{"-i", "branch=master", "-i", "replicas=3", "-i", "env.region=eu"}, "vars.branch=%22master%22&vars.replicas=3&vars.env.region=%22eu%22"),
		)

		Context("when the pipeline exists", func() {

			Context("user and pipeline are part of the main team", func() {
				Context("user is targeting the same team the pipeline belongs to", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", mainPath, queryParams),
								ghttp.RespondWith(http.StatusOK, nil),
							),
						)
					})

					It("unpauses the pipeline", func() {
						Expect(func() {
							flyCmd := exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "awesome-pipeline/branch:master")

							sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
							Expect(err).NotTo(HaveOccurred())

							Eventually(sess).Should(gbytes.Say(`unpaused 'awesome-pipeline/branch:master'`))

							Eventually(sess).Should(gexec.Exit(0))
						}).To(Change(func() int {
							return len(atcServer.ReceivedRequests())
						}).By(2))
					})
				})

				Context("user is NOT targeting the same team the pipeline belongs to", func() {
					BeforeEach(func() {
						atcServer.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", "/api/v1/teams/other-team"),
								ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
									Name: "other-team",
								}),
							),
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("PUT", otherPath),
								ghttp.RespondWith(http.StatusOK, nil),
							),
						)
					})

					It("unpauses the pipeline", func() {
						flyCmd := exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "awesome-pipeline", "--team", "other-team")

						sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say(`unpaused 'awesome-pipeline'`))

						Eventually(sess).Should(gexec.Exit(0))
					})
				})

			})

		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", mainPath),
						ghttp.RespondWith(http.StatusNotFound, nil),
					),
				)
			})

			It("prints helpful message", func() {
				Expect(func() {
					flyCmd := exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "awesome-pipeline")

					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess.Err).Should(gbytes.Say(`pipeline 'awesome-pipeline' not found`))

					Eventually(sess).Should(gexec.Exit(1))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(2))
			})
		})
	})

	Context("when the pipline name or --all is not specified", func() {
		It("errors", func() {
			Expect(func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "unpause-pipeline")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say(`one of the flags '-p, --pipeline' or '-a, --all' is required`))

				Eventually(sess).Should(gexec.Exit(1))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(0))
		})
	})

	Context("when both the pipline name and --all are specified", func() {
		It("errors", func() {
			Expect(func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "awesome-pipeline", "--all")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say(`only one of the flags '-p, --pipeline' or '-a, --all' is allowed`))

				Eventually(sess).Should(gexec.Exit(1))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(0))
		})
	})

	Context("when the pipeline flag is invalid", func() {
		It("fails and print invalid flag error", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "-p", "forbidden/pipelinename")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("error: invalid argument for flag `" + osFlag("p", "pipeline")))
		})
	})

	Context("when the --all flag is passed", func() {
		var (
			somePath      string
			someOtherPath string
			err           error
		)

		BeforeEach(func() {
			somePath, err = atc.Routes.CreatePathForRoute(atc.UnpausePipeline, rata.Params{"pipeline_name": "awesome-pipeline", "team_name": "main"})
			Expect(err).NotTo(HaveOccurred())

			someOtherPath, err = atc.Routes.CreatePathForRoute(atc.UnpausePipeline, rata.Params{"pipeline_name": "more-awesome-pipeline", "team_name": "main"})
			Expect(err).NotTo(HaveOccurred())

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines"),
					ghttp.RespondWithJSONEncoded(200, []atc.Pipeline{
						{Name: "awesome-pipeline", Paused: false, Public: false},
						{Name: "more-awesome-pipeline", Paused: true, Public: false},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", somePath),
					ghttp.RespondWith(http.StatusOK, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", someOtherPath),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)
		})

		It("unpauses every pipeline", func() {
			Expect(func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "unpause-pipeline", "--all")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gbytes.Say(`unpaused 'awesome-pipeline'`))
				Eventually(sess).Should(gbytes.Say(`unpaused 'more-awesome-pipeline'`))

				Eventually(sess).Should(gexec.Exit(0))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(4))
		})

	})
})

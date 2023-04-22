package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("CheckResource", func() {
	var (
		flyCmd              *exec.Cmd
		build               atc.Build
		expectedURL         string
		expectedQueryParams string
	)

	BeforeEach(func() {
		build = atc.Build{
			ID:        123,
			Status:    "started",
			StartTime: 100000000000,
		}

		expectedURL = "/api/v1/teams/main/pipelines/mypipeline/resources/myresource/check"
		expectedQueryParams = "vars.branch=%22master%22"
	})

	Context("when ATC request succeeds", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":{"ref":"fake-ref"},"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-f", "ref:fake-ref", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when version is omitted", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when running without --async", func() {
		var streaming chan struct{}
		var events chan atc.Event

		BeforeEach(func() {
			streaming = make(chan struct{})
			events = make(chan atc.Event)

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
				BuildEventsHandler(123, streaming, events),
			)
		})

		It("checks and watches the build", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))

				AssertEvents(sess, streaming, events)
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(3))
		})
	})

	Context("when specifying the --shallow flag", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null,"shallow":true}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
			)
		})

		It("sends correct check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when specifying multiple versions", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":{"ref1":"fake-ref-1","ref2":"fake-ref-2"},"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
			)
		})

		It("sends correct check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-f", "ref1:fake-ref-1", "-f", "ref2:fake-ref-2", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when pipeline or resource is not found", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
				),
			)
		})

		It("fails with error", func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("pipeline 'mypipeline/branch:master' or resource 'myresource' not found"))
		})
	})

	Context("When resource check returns internal server error", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.RespondWith(http.StatusInternalServerError, "unknown server error"),
				),
			)
		})

		It("outputs error in response body", func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("unknown server error"))

		})
	})

	Context("user is NOT targeting the same team the resource type belongs to", func() {
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
		BeforeEach(func() {
			build = atc.Build{
				ID:        123,
				Status:    "started",
				StartTime: 100000000000,
			}

			expectedURL = "/api/v1/teams/diff-team/pipelines/mypipeline/resources/myresource/check"
			expectedQueryParams = "vars.branch=%22master%22"
		})

		Context("when ATC request succeeds", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
						ghttp.VerifyJSON(`{"from":{"ref":"fake-ref"},"shallow":false}`),
						ghttp.RespondWithJSONEncoded(http.StatusOK, build),
					),
				)
			})

			It("sends check resource request to ATC", func() {
				Expect(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-f", "ref:fake-ref", "-a", "--team", team)
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(3))
			})
		})

		Context("when version is omitted", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
						ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
						ghttp.RespondWithJSONEncoded(http.StatusOK, build),
					),
				)
			})

			It("sends check resource request to ATC", func() {
				Expect(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-a", "--team", team)
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(3))
			})
		})

		Context("when running without --async", func() {
			var streaming chan struct{}
			var events chan atc.Event

			BeforeEach(func() {
				streaming = make(chan struct{})
				events = make(chan atc.Event)

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
						ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
						ghttp.RespondWithJSONEncoded(http.StatusOK, build),
					),
					BuildEventsHandler(123, streaming, events),
				)
			})

			It("checks and watches the build", func() {
				Expect(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--team", team)
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))

					AssertEvents(sess, streaming, events)
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(4))
			})
		})

		Context("when specifying the --shallow flag", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
						ghttp.VerifyJSON(`{"from":null,"shallow":true}`),
						ghttp.RespondWithJSONEncoded(http.StatusOK, build),
					),
				)
			})

			It("sends correct check resource request to ATC", func() {
				Expect(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow", "-a", "--team", team)
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))

					Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(3))
			})
		})

		Context("when specifying multiple versions", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
						ghttp.VerifyJSON(`{"from":{"ref1":"fake-ref-1","ref2":"fake-ref-2"},"shallow":false}`),
						ghttp.RespondWithJSONEncoded(http.StatusOK, build),
					),
				)
			})

			It("sends correct check resource request to ATC", func() {
				Expect(func() {
					flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-f", "ref1:fake-ref-1", "-f", "ref2:fake-ref-2", "-a", "--team", team)
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))

					Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/branch:master/myresource in build 123"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(3))
			})
		})

		Context("when pipeline or resource is not found", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})

			It("fails with error", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow", "--team", team)
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("pipeline 'mypipeline/branch:master' or resource 'myresource' not found"))
			})
		})

		Context("When resource check returns internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
						ghttp.RespondWith(http.StatusInternalServerError, "unknown server error"),
					),
				)
			})

			It("outputs error in response body", func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow", "--team", team)
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("unknown server error"))

			})
		})
	})
})

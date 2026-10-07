package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/v8/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("check-pipeline-resources", func() {
	var (
		flyCmd       *exec.Cmd
		build1       atc.Build
		build2       atc.Build
		resourcesURL string
		check1URL    string
		check2URL    string
	)

	BeforeEach(func() {
		build1 = atc.Build{
			ID:     123,
			Status: "started",
		}
		build2 = atc.Build{
			ID:     124,
			Status: "started",
		}

		resourcesURL = "/api/v1/teams/main/pipelines/mypipeline/resources"
		check1URL = "/api/v1/teams/main/pipelines/mypipeline/resources/resource1/check"
		check2URL = "/api/v1/teams/main/pipelines/mypipeline/resources/resource2/check"
	})

	Context("when running with --async and the pipeline has two resources", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Resource{
						{Name: "resource1"},
						{Name: "resource2"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", check1URL),
					ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build1),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", check2URL),
					ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build2),
				),
			)
		})

		It("triggers checks for all resources and exits without streaming events", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/resource1 in build 123"))
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/resource2 in build 124"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(4))
		})
	})

	Context("when running without --async and the pipeline has one resource", func() {
		var streaming chan struct{}
		var events chan atc.Event

		BeforeEach(func() {
			streaming = make(chan struct{})
			events = make(chan atc.Event)

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Resource{
						{Name: "resource1"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", check1URL),
					ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build1),
				),
				BuildEventsHandler(123, streaming, events),
			)
		})

		It("triggers a check and streams build events for the resource", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/resource1 in build 123"))

				AssertEvents(sess, streaming, events)
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(4))
		})
	})

	Context("when the pipeline has no resources", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Resource{}),
				),
			)
		})

		It("prints no resources found and exits successfully", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("no resources found"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when the pipeline is not found", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
				),
			)
		})

		It("fails with an error", func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline", "-a")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))
			Expect(sess.Err).To(gbytes.Say("resource not found"))
		})
	})

	Context("when a resource check returns not found", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Resource{
						{Name: "resource1"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", check1URL),
					ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
				),
			)
		})

		It("collects the error and exits with failure", func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline", "-a")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))
			Expect(sess.Err).To(gbytes.Say("pipeline 'mypipeline' or resource 'resource1' not found"))
		})
	})

	Context("when specifying the --shallow flag", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Resource{
						{Name: "resource1"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", check1URL),
					ghttp.VerifyJSON(`{"from":null,"shallow":true}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build1),
				),
			)
		})

		It("sends shallow check request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline", "--shallow", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/resource1 in build 123"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(3))
		})
	})

	Context("when a resource check returns internal server error", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Resource{
						{Name: "resource1"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", check1URL),
					ghttp.RespondWith(http.StatusInternalServerError, "unknown server error"),
				),
			)
		})

		It("outputs error in response body and exits with failure", func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline", "-a")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))
			Expect(sess.Err).To(gbytes.Say("unknown server error"))
		})
	})

	Context("when targeting a different team with --team", func() {
		var (
			team             string
			teamResourcesURL string
			teamCheck1URL    string
		)

		BeforeEach(func() {
			team = "other-team"
			teamResourcesURL = "/api/v1/teams/other-team/pipelines/mypipeline/resources"
			teamCheck1URL = "/api/v1/teams/other-team/pipelines/mypipeline/resources/resource1/check"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/%s", team)),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
						Name: team,
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", teamResourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Resource{
						{Name: "resource1"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", teamCheck1URL),
					ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build1),
				),
			)
		})

		It("sends requests to the correct team URLs and exits successfully", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline", "--team", team, "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/resource1 in build 123"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(4))
		})
	})

	Context("when multiple resources are checked and some fail", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []atc.Resource{
						{Name: "resource1"},
						{Name: "resource2"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", check1URL),
					ghttp.RespondWith(http.StatusInternalServerError, "unknown server error"),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", check2URL),
					ghttp.VerifyJSON(`{"from":null,"shallow":false}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build2),
				),
			)
		})

		It("checks all resources without failing fast and exits with failure", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-pipeline-resources", "-p", "mypipeline", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("unknown server error"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(4))
		})
	})
})

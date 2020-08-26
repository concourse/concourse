package integration_test

import (
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("CheckResourceType", func() {
	var (
		flyCmd        *exec.Cmd
		build         atc.Build
		resourceTypes atc.VersionedResourceTypes
	)

	BeforeEach(func() {
		build = atc.Build{
			ID:     123,
			Status: "started",
		}

		resourceTypes = atc.VersionedResourceTypes{{
			ResourceType: atc.ResourceType{
				Name: "myresource",
				Type: "myresourcetype",
			},
		}, {
			ResourceType: atc.ResourceType{
				Name: "myresourcetype",
				Type: "mybaseresourcetype",
			},
		}}
	})

	Context("when version is specified", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.VerifyJSON(`{"from":{"ref":"fake-ref"}}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/myresource", "-f", "ref:fake-ref", "--shallow", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/myresource in build 123"))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when version is omitted", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/myresource", "--shallow", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/myresource in build 123"))
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

			expectedURL := "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
				BuildEventsHandler(123, streaming, events),
			)
		})

		It("checks and watches the build", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/myresource", "--shallow")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/myresource in build 123"))

				AssertEvents(sess, streaming, events)
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(3))
		})
	})

	Context("when recursive check succeeds", func() {
		var parentStreaming chan struct{}
		var parentEvents chan atc.Event

		BeforeEach(func() {
			parentStreaming = make(chan struct{})
			parentEvents = make(chan atc.Event)

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resource-types"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resourceTypes),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/info"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Info{Version: atcVersion, WorkerVersion: workerVersion}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resource-types"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resourceTypes),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresourcetype/check"),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{
						ID:     987,
						Status: "started",
					}),
				),
				BuildEventsHandler(987, parentStreaming, parentEvents),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresource/check"),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, build),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/myresource", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/myresourcetype in build 987"))

				parentEvents <- event.Log{Payload: "sup"}
				Eventually(sess.Out).Should(gbytes.Say("sup"))
				close(parentEvents)

				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/myresource in build 123"))

				Eventually(sess).Should(gexec.Exit(0))
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(7))
		})
	})

	Context("when recursive check fails", func() {
		var parentStreaming chan struct{}
		var parentEvents chan atc.Event

		BeforeEach(func() {
			parentStreaming = make(chan struct{})
			parentEvents = make(chan atc.Event)

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resource-types"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resourceTypes),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/info"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Info{Version: atcVersion, WorkerVersion: workerVersion}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resource-types"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resourceTypes),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresourcetype/check"),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Build{
						ID:     987,
						Status: "started",
					}),
				),
				BuildEventsHandler(987, parentStreaming, parentEvents),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/myresource", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Out).Should(gbytes.Say("checking mypipeline/myresourcetype in build 987"))
				AssertErrorEvents(sess, parentStreaming, parentEvents)
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(6))
		})
	})

	Context("when pipeline or resource-type is not found", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
				),
			)
		})

		It("fails with error", func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/myresource", "--shallow")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("pipeline 'mypipeline' or resource-type 'myresource' not found"))
		})
	})

	Context("When resource-type check returns internal server error", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWith(http.StatusInternalServerError, "unknown server error"),
				),
			)
		})

		It("outputs error in response body", func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource-type", "-r", "mypipeline/myresource", "--shallow")
			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("unknown server error"))
		})
	})
})

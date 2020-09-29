package integration_test

import (
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("CheckResource", func() {
	var (
		flyCmd              *exec.Cmd
		check               atc.Check
		resource            atc.Resource
		resourceTypes       atc.VersionedResourceTypes
		expectedURL         string
		expectedQueryParams string
		expectedHeaders     ui.TableRow
	)

	BeforeEach(func() {
		check = atc.Check{
			ID:         123,
			Status:     "started",
			CreateTime: 100000000000,
		}

		resource = atc.Resource{
			Name: "myresource",
			Type: "myresourcetype",
		}

		resourceTypes = atc.VersionedResourceTypes{{
			ResourceType: atc.ResourceType{
				Name: "myresourcetype",
				Type: "mybaseresourcetype",
			},
		}}

		expectedURL = "/api/v1/teams/main/pipelines/mypipeline/resources/myresource/check"
		expectedQueryParams = "instance_vars=%7B%22branch%22%3A%22master%22%7D"

		expectedHeaders = ui.TableRow{
			{Contents: "id", Color: color.New(color.Bold)},
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "status", Color: color.New(color.Bold)},
			{Contents: "check_error", Color: color.New(color.Bold)},
		}
	})

	Context("when ATC request succeeds", func() {

		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":{"ref":"fake-ref"}}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, check),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-f", "ref:fake-ref", "-a", "--shallow")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "123"},
							{Contents: "myresource"},
							{Contents: "started"},
							{Contents: ""},
						},
					},
				}))

			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when version is omitted", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/main/pipelines/mypipeline/resources/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, check),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "123"},
							{Contents: "myresource"},
							{Contents: "started"},
							{Contents: ""},
						},
					},
				}))

			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(2))
		})
	})

	Context("when the check succeed", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/main/pipelines/mypipeline/resources/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, check),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/checks/123"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Check{
						ID:         123,
						Status:     "succeeded",
						CreateTime: 100000000000,
						StartTime:  100000000000,
						EndTime:    100000000000,
					}),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "123"},
							{Contents: "myresource"},
							{Contents: "succeeded"},
							{Contents: ""},
						},
					},
				}))

			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(3))
		})
	})

	Context("when the check fail", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/main/pipelines/mypipeline/resources/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, check),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/checks/123"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Check{
						ID:         123,
						Status:     "errored",
						CreateTime: 100000000000,
						StartTime:  100000000000,
						EndTime:    100000000000,
						CheckError: "some-check-error",
					}),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "--shallow")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Eventually(sess.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "123"},
							{Contents: "myresource"},
							{Contents: "errored"},
							{Contents: "some-check-error"},
						},
					},
				}))

			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(3))
		})
	})

	Context("when recursive check succeeds", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resources/myresource", expectedQueryParams),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resource),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resource-types", expectedQueryParams),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resourceTypes),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/info"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Info{Version: atcVersion, WorkerVersion: workerVersion}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resource-types", expectedQueryParams),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resourceTypes),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresourcetype/check", expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Check{
						ID:     987,
						Status: "started",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/checks/987"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Check{
						ID:         987,
						Status:     "succeeded",
						CreateTime: 100000000000,
						StartTime:  100000000000,
						EndTime:    100000000000,
						CheckError: "",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v1/teams/main/pipelines/mypipeline/resources/myresource/check", expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, check),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "987"},
							{Contents: "myresourcetype"},
							{Contents: "succeeded"},
							{Contents: ""},
						},
					},
				}))

				Eventually(sess.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "123"},
							{Contents: "myresource"},
							{Contents: "started"},
							{Contents: ""},
						},
					},
				}))

			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(8))
		})
	})

	Context("when recursive check fails", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resources/myresource", expectedQueryParams),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resource),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resource-types", expectedQueryParams),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resourceTypes),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/info"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Info{Version: atcVersion, WorkerVersion: workerVersion}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/teams/main/pipelines/mypipeline/resource-types", expectedQueryParams),
					ghttp.RespondWithJSONEncoded(http.StatusOK, resourceTypes),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/api/v1/teams/main/pipelines/mypipeline/resource-types/myresourcetype/check", expectedQueryParams),
					ghttp.VerifyJSON(`{"from":null}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Check{
						ID:     987,
						Status: "started",
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/checks/987"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Check{
						ID:         987,
						Status:     "errored",
						CreateTime: 100000000000,
						StartTime:  100000000000,
						EndTime:    100000000000,
						CheckError: "failed to check",
					}),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Eventually(sess.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "987"},
							{Contents: "myresourcetype"},
							{Contents: "errored"},
							{Contents: "failed to check"},
						},
					},
				}))

			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(7))
		})
	})

	Context("when specifying multiple versions", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQueryParams),
					ghttp.VerifyJSON(`{"from":{"ref1":"fake-ref-1","ref2":"fake-ref-2"}}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, check),
				),
			)
		})

		It("sends correct check resource request to ATC", func() {
			Expect(func() {
				flyCmd = exec.Command(flyPath, "-t", targetName, "check-resource", "-r", "mypipeline/branch:master/myresource", "-f", "ref1:fake-ref-1", "-f", "ref2:fake-ref-2", "--shallow", "-a")
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "123"},
							{Contents: "myresource"},
							{Contents: "started"},
						},
					},
				}))
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
})

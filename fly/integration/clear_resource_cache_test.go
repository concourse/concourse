package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/concourse/concourse/v8/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("clear-resource-cache", func() {
	var (
		initialRequests     int
		expectedQueryParams []string
		expectedURL         = "/api/v1/teams/main/pipelines/some-pipeline/resources/some-resource/cache"
		stdin               io.Writer
		args                []string
		sess                *gexec.Session
	)

	BeforeEach(func() {
		stdin = nil
		args = []string{}
	})

	JustBeforeEach(func() {
		var err error
		initialRequests = len(atcServer.ReceivedRequests())

		flyCmd := exec.Command(flyPath, append([]string{"-t", targetName, "clear-resource-cache"}, args...)...)
		stdin, err = flyCmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())

		sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	})

	yes := func() {
		Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
		fmt.Fprintf(stdin, "y\r")
	}

	no := func() {
		Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
		fmt.Fprintf(stdin, "n\r")
	}

	DescribeTableSubtree("rejects invalid selector combinations",
		func(flags []string, message string) {
			BeforeEach(func() { args = flags })
			It("rejects the flags before making API requests", func() {
				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say(message))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(initialRequests))
			})
		},
		Entry("missing selectors", []string{}, "must specify either --pipeline or --resource"),
		Entry("both selectors", []string{"--pipeline", "some-pipeline", "--resource", "some-pipeline/some-resource"}, "cannot specify both --pipeline and --resource"),
		Entry("pipeline and version", []string{"--pipeline", "some-pipeline", "--version", "ref:fake-ref"}, "--version requires --resource"),
	)

	Context("when a pipeline is specified", func() {
		const resourcesURL = "/api/v1/teams/main/pipelines/some-pipeline/resources"

		listResources := func(url, query string, resources []atc.Resource) http.HandlerFunc {
			return ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", url, query),
				ghttp.RespondWithJSONEncoded(http.StatusOK, resources),
			)
		}

		deleteCache := func(url, query string, removed int64) http.HandlerFunc {
			return ghttp.CombineHandlers(
				ghttp.VerifyRequest("DELETE", url, query),
				ghttp.VerifyJSON(`{"version":null}`),
				ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearResourceCacheResponse{CachesRemoved: removed}),
			)
		}

		BeforeEach(func() {
			args = []string{"--pipeline", "some-pipeline"}
		})

		Context("when an instanced pipeline and team are selected", func() {
			BeforeEach(func() {
				args = []string{"-p", "some-pipeline/branch:main", "--team", "custom-team"}
				const url = "/api/v1/teams/custom-team/pipelines/some-pipeline/resources"
				const query = "vars.branch=%22main%22"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/teams/custom-team"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{Name: "custom-team"}),
					),
					listResources(url, query, []atc.Resource{{Name: "first"}, {Name: "second"}}),
					deleteCache(url+"/first/cache", query, 2),
					deleteCache(url+"/second/cache", query, 3),
				)
			})

			It("clears every resource in an instanced pipeline on the selected team", func() {
				yes()
				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(gbytes.Say("5 caches removed"))
				Expect(strings.Count(string(sess.Out.Contents()), "are you sure?")).To(Equal(1))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(initialRequests + 5))
			})

		})

		Context("when confirmation is declined", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(listResources(resourcesURL, "", []atc.Resource{{Name: "first"}}))
			})

			It("exits without deleting any caches", func() {
				no()
				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(gbytes.Say("bailing out"))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(initialRequests + 2))
			})
		})

		Context("when the pipeline is empty", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(listResources(resourcesURL, "", []atc.Resource{}))
			})

			It("reports that there are no resources without prompting", func() {
				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(gbytes.Say("no resources found"))
				Expect(sess.Out).NotTo(gbytes.Say("are you sure"))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(initialRequests + 2))
			})
		})

		Context("when listing resources fails", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", resourcesURL),
					ghttp.RespondWith(http.StatusInternalServerError, ""),
				))
			})

			It("returns the listing error without prompting", func() {
				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("Unexpected Response"))
				Expect(sess.Out).NotTo(gbytes.Say("are you sure"))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(initialRequests + 2))
			})
		})

		Context("when one deletion fails", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					listResources(resourcesURL, "", []atc.Resource{{Name: "first"}, {Name: "second"}}),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", resourcesURL+"/first/cache"),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
					deleteCache(resourcesURL+"/second/cache", "", 2),
				)
			})

			It("clears the remaining resource and reports the failure", func() {
				yes()
				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("error clearing cache for 'first'"))
				Expect(sess.Out).To(gbytes.Say("2 caches removed"))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(initialRequests + 4))
			})
		})
	})

	Context("when resource and a version are specified", func() {
		Context("team is not specified", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.VerifyJSON(`{"version":{"ref":"fake-ref"}}`),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearResourceCacheResponse{CachesRemoved: 1}),
					),
				)
			})

			BeforeEach(func() {
				args = append(args, "-r", "some-pipeline/some-resource", "-v", "ref:fake-ref")
			})

			It("warns this could be a dangerous command", func() {
				Eventually(sess).Should(gbytes.Say("this will remove the resource cache\\(s\\) for `some-pipeline/some-resource` with version: `{\"ref\":\"fake-ref\"}`"))
			})

			It("succeeds with one deletion", func() {
				Expect(func() {
					yes()
					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess).Should(gbytes.Say("1 caches removed"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(2))
			})

			It("bails out if the user negate the command", func() {
				no()
				Eventually(sess).Should(gbytes.Say(`bailing out`))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})
		Context("team is specified", func() {
			var teamName = "custom-team"
			var expectedURL = fmt.Sprintf("/api/v1/teams/%s/pipelines/some-pipeline/resources/some-resource/cache", teamName)

			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/%s", teamName)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
							Name: teamName,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.VerifyJSON(`{"version":{"ref":"fake-ref"}}`),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearResourceCacheResponse{CachesRemoved: 1}),
					),
				)
			})

			BeforeEach(func() {
				args = append(args, "-r", "some-pipeline/some-resource", "-v", "ref:fake-ref", "--team", teamName)
			})

			It("succeeds with one deletion", func() {
				Expect(func() {
					yes()
					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess).Should(gbytes.Say("1 caches removed"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(3))
			})
		})
	})

	Context("when only a resource is specified", func() {
		Context("when the resource exists", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearResourceCacheResponse{CachesRemoved: 1}),
					),
				)
			})

			BeforeEach(func() {
				args = append(args, "-r", "some-pipeline/some-resource")
			})

			It("succeeds with one deletion", func() {
				Expect(func() {
					yes()
					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess).Should(gbytes.Say("1 caches removed"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(2))
			})
		})

		Context("when the resource exist and team is specified", func() {
			var teamName = "custom-team"
			var expectedURL = fmt.Sprintf("/api/v1/teams/%s/pipelines/some-pipeline/resources/some-resource/cache", teamName)

			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/api/v1/teams/%s", teamName)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Team{
							Name: teamName,
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearResourceCacheResponse{CachesRemoved: 1}),
					),
				)
			})

			BeforeEach(func() {
				args = append(args, "-r", "some-pipeline/some-resource", "--team", teamName)
			})

			It("succeeds with one deletion", func() {
				Expect(func() {
					yes()
					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess).Should(gbytes.Say("1 caches removed"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(3))
			})

			It("bails out if the user negate the command", func() {
				no()
				Eventually(sess).Should(gbytes.Say(`bailing out`))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("when the resource does not exist", func() {
			var expectedURL = "/api/v1/teams/main/pipelines/some-pipeline/resources/no-existing-resource/cache"

			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})

			BeforeEach(func() {
				args = append(args, "-r", "some-pipeline/no-existing-resource")
			})

			It("writes that it did not exist", func() {
				Expect(func() {
					yes()
					Eventually(sess).Should(gexec.Exit(1))
					Eventually(sess.Err).Should(gbytes.Say("resource not found"))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(2))
			})
		})

		Context("and the api returns an unexpected status code", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, strings.Join(expectedQueryParams, "&")),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			BeforeEach(func() {
				args = append(args, "-r", "some-pipeline/some-resource")
			})

			It("writes an error message to stderr", func() {
				Expect(func() {
					yes()
					Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
					Eventually(sess).Should(gexec.Exit(1))
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(2))
			})
		})
	})
})

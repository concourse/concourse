package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("clear-versions", func() {
		var (
			sharedResourcesStatus int
			deleteVersionsStatus  int
			stdin                 io.Writer
			args                  []string
			sess                  *gexec.Session
		)

		BeforeEach(func() {
			stdin = nil
			args = []string{}
		})

		JustBeforeEach(func() {
			var err error

			flyCmd := exec.Command(flyPath, append([]string{"-t", targetName, "clear-versions"}, args...)...)
			stdin, err = flyCmd.StdinPipe()
			Expect(err).ToNot(HaveOccurred())

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
		})

		yes := func() {
			Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
			fmt.Fprintf(stdin, "y\n")
		}

		no := func() {
			Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
			fmt.Fprintf(stdin, "n\n")
		}

		Context("when a resource or resource type is not specified", func() {
			It("asks the user to specify a resource or resource type", func() {
				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("please specify one of the required flags --resource or --resource-type"))
			})
		})

		Context("when a resource is specified", func() {
			var (
				expectedDeleteURL = "/api/v1/teams/main/pipelines/some-pipeline/resources/some-resource/versions"
				expectedSharedURL = "/api/v1/teams/main/pipelines/some-pipeline/resources/some-resource/shared"
			)

			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedSharedURL),
						ghttp.RespondWithJSONEncoded(sharedResourcesStatus, atc.ResourcesAndTypes{
							Resources: atc.ResourceIdentifiers{
								{
									Name:         "some-resource",
									PipelineName: "some-pipeline",
									TeamName:     "some-team",
								},
								{
									Name:         "other-resource",
									PipelineName: "some-pipeline",
									TeamName:     "some-team",
								},
								{
									Name:         "other-resource-2",
									PipelineName: "other-pipeline",
									TeamName:     "other-team",
								},
							},
							ResourceTypes: atc.ResourceIdentifiers{
								{
									Name:         "some-resource-type",
									PipelineName: "some-pipeline",
									TeamName:     "some-team",
								},
							},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedDeleteURL),
						ghttp.RespondWithJSONEncoded(deleteVersionsStatus, atc.ClearVersionsResponse{VersionsRemoved: 1}),
					),
				)
			})

			BeforeEach(func() {
				args = append(args, "--resource", "some-pipeline/some-resource")
			})

			Context("when the resource exists and delete succeeds", func() {
				BeforeEach(func() {
					sharedResourcesStatus = http.StatusOK
					deleteVersionsStatus = http.StatusOK
				})

				It("warns any shared resources/resource-types that the deletion will affect (because of shared version history)", func() {
					Eventually(sess).Should(gbytes.Say(`!!! this will clear the version histories for the following resources:
- some-team/some-pipeline/some-resource
- some-team/some-pipeline/other-resource
- other-team/other-pipeline/other-resource-2

and the following resource types:
- some-team/some-pipeline/some-resource-type`))
				})

				It("succeeds with deletion", func() {
					yes()
					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess).Should(gbytes.Say("1 versions removed"))
				})

				It("bails out when user says no", func() {
					no()
					Eventually(sess).Should(gbytes.Say(`bailing out`))
					Eventually(sess).ShouldNot(gbytes.Say("versions removed"))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("when deleting the versions fails", func() {
				BeforeEach(func() {
					sharedResourcesStatus = http.StatusOK
					deleteVersionsStatus = http.StatusInternalServerError
				})

				It("fails to delete versions", func() {
					yes()
					Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
					Expect(sess.ExitCode()).ToNot(Equal(0))
				})
			})

			Context("when the resource is not found when fetching shared resources/resource types", func() {
				BeforeEach(func() {
					sharedResourcesStatus = http.StatusNotFound
					deleteVersionsStatus = http.StatusOK
				})

				It("fails to delete versions", func() {
					Eventually(sess.Err).Should(gbytes.Say("resource 'some-resource' is not found"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("when fetching shared resources/resource-types returns unexpected status code", func() {
				BeforeEach(func() {
					sharedResourcesStatus = http.StatusInternalServerError
					deleteVersionsStatus = http.StatusOK
				})

				It("fails to delete versions", func() {
					Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Context("when a resource type is specified", func() {
			var (
				expectedDeleteURL = "/api/v1/teams/main/pipelines/some-pipeline/resource-types/some-resource-type/versions"
				expectedSharedURL = "/api/v1/teams/main/pipelines/some-pipeline/resource-types/some-resource-type/shared"
			)

			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedSharedURL),
						ghttp.RespondWithJSONEncoded(sharedResourcesStatus, atc.ResourcesAndTypes{
							Resources: atc.ResourceIdentifiers{
								{
									Name:         "some-resource",
									PipelineName: "some-pipeline",
									TeamName:     "some-team",
								},
							},
							ResourceTypes: atc.ResourceIdentifiers{
								{
									Name:         "some-resource-type",
									PipelineName: "some-pipeline",
									TeamName:     "some-team",
								},
								{
									Name:         "other-resource-type",
									PipelineName: "some-pipeline",
									TeamName:     "some-team",
								},
								{
									Name:         "other-resource-type-2",
									PipelineName: "other-pipeline",
									TeamName:     "other-team",
								},
							},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedDeleteURL),
						ghttp.RespondWithJSONEncoded(deleteVersionsStatus, atc.ClearVersionsResponse{VersionsRemoved: 2}),
					),
				)
			})

			BeforeEach(func() {
				args = append(args, "--resource-type", "some-pipeline/some-resource-type")
			})

			Context("when the resource type exists and delete succeeds", func() {
				BeforeEach(func() {
					sharedResourcesStatus = http.StatusOK
					deleteVersionsStatus = http.StatusOK
				})

				It("warns any shared resources/resource-types that the deletion will affect (because of shared version history)", func() {
					Eventually(sess).Should(gbytes.Say(`!!! this will clear the version histories for the following resources:
- some-team/some-pipeline/some-resource

and the following resource types:
- some-team/some-pipeline/some-resource-type
- some-team/some-pipeline/other-resource-type
- other-team/other-pipeline/other-resource-type-2`))
				})

				It("succeeds with deletion", func() {
					yes()
					Eventually(sess).Should(gexec.Exit(0))
					Eventually(sess).Should(gbytes.Say("2 versions removed"))
				})

				It("bails out when user says no", func() {
					no()
					Eventually(sess).Should(gbytes.Say(`bailing out`))
					Eventually(sess).ShouldNot(gbytes.Say("versions removed"))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("when deleting the versions fails", func() {
				BeforeEach(func() {
					sharedResourcesStatus = http.StatusOK
					deleteVersionsStatus = http.StatusInternalServerError
				})

				It("fails to delete versions", func() {
					yes()
					Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
					Expect(sess.ExitCode()).ToNot(Equal(0))
				})
			})

			Context("when the resource type is not found when fetching shared resources/resource types", func() {
				BeforeEach(func() {
					sharedResourcesStatus = http.StatusNotFound
					deleteVersionsStatus = http.StatusOK
				})

				It("fails to delete versions", func() {
					Eventually(sess.Err).Should(gbytes.Say("resource type 'some-resource-type' is not found"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})

			Context("when fetching shared resources/resource-types returns unexpected status code", func() {
				BeforeEach(func() {
					sharedResourcesStatus = http.StatusInternalServerError
					deleteVersionsStatus = http.StatusOK
				})

				It("fails to delete versions", func() {
					Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
					Eventually(sess).Should(gexec.Exit(1))
				})
			})
		})

		Context("when both resource and resource type is specified", func() {
			BeforeEach(func() {
				args = append(args, "--resource", "some-pipeline/some-resource", "--resource-type", "some-pipeline/some-resource-type")
			})

			It("errors", func() {
				Eventually(sess.Err).Should(gbytes.Say(fmt.Sprintf("can specify only one of --resource or --resource-type\n")))
				Expect(sess.ExitCode()).ToNot(Equal(0))
			})
		})
	})
})

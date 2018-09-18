package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("clear-task-cache", func() {
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

			flyCmd := exec.Command(flyPath, append([]string{"-t", targetName, "clear-task-cache"}, args...)...)
			stdin, err = flyCmd.StdinPipe()
			Expect(err).NotTo(HaveOccurred())

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when a job is not specified", func() {
			It("asks the user to specify a job", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "clear-task-cache", "-s", "some-task-step")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("j", "job") + "' was not specified"))
			})
		})

		Context("when a step is not specified", func() {
			It("asks the user to specify a step", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "clear-task-cache", "-j", "some-pipeline/some-job")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))

				Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("s", "step") + "' was not specified"))
			})
		})

		Context("when specifying a job without a pipeline name", func() {
			It("asks the user to specify a pipeline name", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "clear-task-cache", "-s", "some-task-step", "-j", "myjob")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("error: invalid argument for flag `" + osFlag("j", "job")))
				Expect(sess.Err).To(gbytes.Say(`argument format should be <pipeline>/<job>`))
			})
		})

		Context("when a job and step name are specified", func() {
			BeforeEach(func() {
				args = append(args, "-j", "some-pipeline/some-job", "-s", "some-step-name")
			})

			yes := func() {
				Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
				fmt.Fprintf(stdin, "y\n")
			}

			no := func() {
				Eventually(sess).Should(gbytes.Say(`are you sure\? \[yN\]: `))
				fmt.Fprintf(stdin, "n\n")
			}

			It("warns that it's about to do bad things", func() {
				Eventually(sess).Should(gbytes.Say("!!! this will remove the task cache\\(s\\) for `some-pipeline/some-job`, task step `some-step-name`"))
			})

			It("bails out if the user says no", func() {
				no()
				Eventually(sess).Should(gbytes.Say(`bailing out`))
				Eventually(sess).Should(gexec.Exit(0))
			})

			Context("when the task step exists", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job/tasks/some-step-name/cache"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearTaskCacheResponse{CachesRemoved: 1}),
						),
					)
				})

				It("succeeds if the user says yes", func() {
					yes()
					Eventually(sess).Should(gbytes.Say("1 caches removed"))
					Eventually(sess).Should(gexec.Exit(0))
				})

				Context("when run noninteractively", func() {
					BeforeEach(func() {
						args = append(args, "-n")
					})

					It("destroys the task step cache without confirming", func() {
						Eventually(sess).Should(gbytes.Say("1 caches removed"))
						Eventually(sess).Should(gexec.Exit(0))
					})
				})

				Context("and a cache path is specified", func() {
					BeforeEach(func() {
						args = append(args, "-c", "path/to/cache")
					})

					It("succeeds if the user says yes", func() {
						yes()
						Eventually(sess).Should(gbytes.Say("1 caches removed"))
						Eventually(sess).Should(gexec.Exit(0))
					})

				})
			})

			Context("and the task step does not exist", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job/tasks/some-step-name/cache"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ClearTaskCacheResponse{CachesRemoved: 0}),
						),
					)
				})

				It("writes that it did not exist and exits successfully", func() {
					yes()
					Eventually(sess).Should(gbytes.Say("0 caches removed"))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("and the api returns an unexpected status code", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job/tasks/some-step-name/cache"),
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
		})
	})
})

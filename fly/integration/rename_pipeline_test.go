package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("RenamePipeline", func() {
	var (
		expectedURL         string
		expectedQueryParams string
		expectedStatusCode  int
		newName             string
	)

	BeforeEach(func() {
		expectedURL = "/api/v1/teams/main/pipelines/some-pipeline/rename"
		expectedStatusCode = http.StatusNoContent
		newName = "brandnew"
	})

	JustBeforeEach(func() {
		atcServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", expectedURL, expectedQueryParams),
				ghttp.VerifyJSON(fmt.Sprintf(`{"name":%q}`, newName)),
				ghttp.RespondWith(expectedStatusCode, ""),
			),
		)
	})

	Context("when not specifying a pipeline name", func() {
		It("fails and says you should provide a pipeline name", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-n", "some-new-name")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("o", "old-name") + "' was not specified"))
		})
	})

	Context("when specifying a new pipeline name with a '/' character in it", func() {
		It("fails and says '/' characters are not allowed", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline", "-n", "forbidden/pipelinename")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say("error: pipeline name cannot contain '/'"))
		})
	})

	Context("when the pipeline flag is invalid", func() {
		It("fails and print invalid flag error", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "forbidden/pipelinename", "-n", "some-new-name")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say("error: invalid argument for flag `" + osFlag("o", "old-name")))
		})
	})

	Context("when not specifying a new name", func() {
		It("fails and says you should provide a new name for the pipeline", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(1))

			Expect(sess.Err).To(gbytes.Say("error: the required flag `" + osFlag("n", "new-name") + "' was not specified"))
		})
	})

	Context("when all the inputs are provided", func() {

		BeforeEach(func() {
			expectedQueryParams = "instance_vars=%7B%22branch%22%3A%22master%22%7D"
		})

		It("successfully renames the pipeline to the provided name", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline/branch:master", "-n", newName)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(atcServer.ReceivedRequests()).To(HaveLen(5))
			Expect(sess.Out).To(gbytes.Say(fmt.Sprintf("pipeline successfully renamed to %s", newName)))
		})

		Context("when the pipeline is not found", func() {
			BeforeEach(func() {
				expectedStatusCode = http.StatusNotFound
			})

			It("returns an error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline/branch:master", "-n", newName)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(5))
				Expect(sess.Err).To(gbytes.Say("pipeline 'some-pipeline/branch:master' not found"))
			})
		})

		Context("when an error occurs", func() {
			BeforeEach(func() {
				expectedStatusCode = http.StatusTeapot
			})

			It("returns an error", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "rename-pipeline", "-o", "some-pipeline/branch:master", "-n", newName)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Expect(atcServer.ReceivedRequests()).To(HaveLen(5))
				Expect(sess.Err).To(gbytes.Say("418 I'm a teapot"))
			})
		})
	})
})

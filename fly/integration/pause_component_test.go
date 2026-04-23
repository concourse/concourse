package integration_test

import (
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

var _ = Describe("pause-component", func() {
	Context("when a component name is specified", func() {
		var (
			path string
			err  error
		)

		BeforeEach(func() {
			path, err = atc.Routes.CreatePathForRoute(atc.PauseComponent, rata.Params{"component_name": "scheduler"})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the component exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", path),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)
			})

			It("pauses the component", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "pause-component", "-n", "scheduler")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gbytes.Say("paused 'scheduler'"))
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("when the component does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", path),
						ghttp.RespondWith(http.StatusNotFound, nil),
					),
				)
			})

			It("prints an error message", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "pause-component", "-n", "scheduler")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("component 'scheduler' not found"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("when the user is forbidden", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", path),
						ghttp.RespondWith(http.StatusForbidden, nil),
					),
				)
			})

			It("returns an error about needing owner role", func() {
				flyCmd := exec.Command(flyPath, "-t", targetName, "pause-component", "-n", "scheduler")

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("must be an owner of the 'main' team to interact with components"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})

	Context("when multiple component names are specified", func() {
		var (
			schedulerPath string
			trackerPath   string
			err           error
		)

		BeforeEach(func() {
			schedulerPath, err = atc.Routes.CreatePathForRoute(atc.PauseComponent, rata.Params{"component_name": "scheduler"})
			Expect(err).NotTo(HaveOccurred())

			trackerPath, err = atc.Routes.CreatePathForRoute(atc.PauseComponent, rata.Params{"component_name": "tracker"})
			Expect(err).NotTo(HaveOccurred())

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", schedulerPath),
					ghttp.RespondWith(http.StatusOK, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", trackerPath),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)
		})

		It("pauses each component", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "pause-component", "-n", "scheduler", "-n", "tracker")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gbytes.Say("paused 'scheduler'"))
			Eventually(sess).Should(gbytes.Say("paused 'tracker'"))
			Eventually(sess).Should(gexec.Exit(0))
		})
	})

	Context("when --all is specified", func() {
		var (
			path string
			err  error
		)

		BeforeEach(func() {
			path, err = atc.Routes.CreatePathForRoute(atc.PauseAllComponents, nil)
			Expect(err).NotTo(HaveOccurred())

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", path),
					ghttp.RespondWith(http.StatusOK, nil),
				),
			)
		})

		It("pauses all components", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "pause-component", "--all")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gbytes.Say("all components paused"))
			Eventually(sess).Should(gexec.Exit(0))
		})
	})

	Context("when neither --name or --all is specified", func() {
		It("errors", func() {
			flyCmd := exec.Command(flyPath, "-t", targetName, "pause-component")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("--name or --all must be provided"))
			Eventually(sess).Should(gexec.Exit(1))
		})
	})
})

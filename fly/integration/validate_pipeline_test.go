package integration_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fly CLI", func() {
	Describe("validate-pipeline", func() {
		It("returns valid on valid configuration", func() {
			flyCmd := exec.Command(
				flyPath,
				"validate-pipeline",
				"-c", "fixtures/testConfigValid.yml",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gbytes.Say("looks good"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

		It("returns valid on valid configuration to stdout", func() {
			flyCmd := exec.Command(
				flyPath,
				"validate-pipeline",
				"-c", "fixtures/testConfigValid.yml",
				"-o",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gbytes.Say("groups:"))
			Eventually(sess).Should(gbytes.Say("jobs:"))
			Eventually(sess).Should(gbytes.Say("resources:"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

		It("returns valid on templated configuration with variables", func() {
			flyCmd := exec.Command(
				flyPath,
				"validate-pipeline",
				"-c", "fixtures/vars-pipeline.yml",
				"-l", "fixtures/vars-pipeline-params-types.yml",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gbytes.Say("looks good"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

		It("returns invalid on validation error", func() {
			flyCmd := exec.Command(
				flyPath,
				"validate-pipeline",
				"-c", "fixtures/testConfigError.yml",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("WARNING:"))
			Eventually(sess.Err).Should(gbytes.Say("  - invalid resources:"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))

			Expect(sess.Err).To(gbytes.Say("configuration invalid"))
		})

		It("returns invalid on duplicate keys", func() {
			flyCmd := exec.Command(
				flyPath,
				"validate-pipeline",
				"-c", "fixtures/testConfigDuplicate.yml",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Err).Should(gbytes.Say("error parsing yaml before applying templates"))
			Eventually(sess.Err).Should(gbytes.Say("key \"resources\" already set in map"))

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(1))
		})

		It("returns valid on a pipeline with unknown keys", func() {
			flyCmd := exec.Command(
				flyPath,
				"validate-pipeline",
				"-c", "fixtures/testConfigUnknownKeys.yml",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})

		When("strict", func() {
			It("returns invalid on validation warning", func() {
				flyCmd := exec.Command(
					flyPath,
					"validate-pipeline",
					"-c", "fixtures/testConfigWarning.yml",
					"--strict",
				)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("DEPRECATION WARNING:"))
				Eventually(sess.Err).Should(gbytes.Say("  - jobs.some-job.plan"))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))

				Expect(sess.Err).To(gbytes.Say("configuration invalid"))
			})

			It("returns invalid on unknown keys", func() {
				flyCmd := exec.Command(
					flyPath,
					"validate-pipeline",
					"-c", "fixtures/testConfigUnknownKeys.yml",
					"--strict",
				)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess.Err).Should(gbytes.Say("json: unknown field \"user_key\""))

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(1))
			})
		})

		It("returns valid on a pipeline that contains var_sources", func() {
			flyCmd := exec.Command(
				flyPath,
				"validate-pipeline",
				"-c", "fixtures/var_sources_pipeline.yml",
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))
		})
	})
})

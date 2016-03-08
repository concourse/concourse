package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"

	"github.com/concourse/fly/version"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Version Checks", func() {
	// patch version
	var (
		flyVersion string
		flySession *gexec.Session
	)

	BeforeEach(func() {
		atcServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/api/v1/containers"),
				ghttp.RespondWith(http.StatusOK, "[]"),
			),
		)
	})

	JustBeforeEach(func() {
		flyPath, err := gexec.Build(
			"github.com/concourse/fly",
			"-ldflags", fmt.Sprintf("-X github.com/concourse/fly/version.Version=%s", flyVersion),
		)
		Expect(err).NotTo(HaveOccurred())

		flyCmd := exec.Command(flyPath, "-t", targetName, "containers")

		flySession, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("when the client and server differ by a patch version", func() {
		BeforeEach(func() {
			major, minor, patch, err := version.GetSemver(atcVersion)
			Expect(err).NotTo(HaveOccurred())

			flyVersion = fmt.Sprintf("%d.%d.%d", major, minor, patch+1)
		})

		It("warns the user that there is a difference", func() {
			Eventually(flySession.Err).Should(gbytes.Say("fly version does not match the target version, please re-sync it"))
			Eventually(flySession).Should(gexec.Exit(0))
		})
	})

	// when then match
	Describe("when the client and server are the same version", func() {
		BeforeEach(func() {
			flyVersion = atcVersion
		})

		It("warns the user that there is a difference", func() {
			Eventually(flySession).Should(gexec.Exit(0))
			Expect(flySession.Err).ShouldNot(gbytes.Say("version"))
		})
	})

	// minor version
	Describe("when the client and server differ by a minor version", func() {
		BeforeEach(func() {
			major, minor, patch, err := version.GetSemver(atcVersion)
			Expect(err).NotTo(HaveOccurred())

			flyVersion = fmt.Sprintf("%d.%d.%d", major, minor+1, patch)
		})

		It("error and tell the user to upgrade", func() {
			Eventually(flySession).Should(gexec.Exit(1))
			Expect(flySession.Err).Should(gbytes.Say("fly version is out of sync with the target. run the following command to re-sync it:"))
			Expect(flySession.Err).Should(gbytes.Say(`    fly -t \(alias\) sync`))
		})
	})

	// major version (same as minor)
	Describe("when the client and server differ by a major version", func() {
		BeforeEach(func() {
			major, minor, patch, err := version.GetSemver(atcVersion)
			Expect(err).NotTo(HaveOccurred())

			flyVersion = fmt.Sprintf("%d.%d.%d", major+1, minor, patch)
		})

		It("error and tell the user to upgrade", func() {
			Eventually(flySession).Should(gexec.Exit(1))
			Expect(flySession.Err).Should(gbytes.Say("fly version is out of sync with the target. run the following command to re-sync it:"))
			Expect(flySession.Err).Should(gbytes.Say(`    fly -t \(alias\) sync`))
		})
	})

	// dev version
	Describe("when the client is a development version", func() {
		BeforeEach(func() {
			flyVersion = version.Version
		})

		It("never complains", func() {
			Eventually(flySession).Should(gexec.Exit(0))
			Expect(flySession.Err).ShouldNot(gbytes.Say("version"))
		})
	})
})

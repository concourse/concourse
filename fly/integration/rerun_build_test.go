package integration_test

import (
	"net/http"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	"github.com/concourse/concourse/v8/atc"
)

var _ = Describe("rerun-build", func() {
	It("reruns the build for the specified pipeline instance", func() {
		rerunPath, err := atc.Routes.CreatePathForRoute(atc.RerunJobBuild, rata.Params{
			"build_name":    "42",
			"job_name":      "awesome-job",
			"pipeline_name": "awesome-pipeline",
			"team_name":     "main",
		})
		Expect(err).NotTo(HaveOccurred())

		atcServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", rerunPath, "vars.branch=%22master%22"),
				ghttp.RespondWithJSONEncoded(http.StatusCreated, atc.Build{ID: 57, Name: "43"}),
			),
		)

		flyCmd := exec.Command(flyPath, "-t", targetName, "rerun-build", "-j", "awesome-pipeline/awesome-job", "-b", "42", "-i", "branch=master")
		sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(sess).Should(gbytes.Say(`started awesome-pipeline/branch:master/awesome-job #43`))
		Eventually(sess).Should(gexec.Exit(0))
	})
})

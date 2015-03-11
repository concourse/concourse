package integration_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
)

var _ = Describe("Fly CLI", func() {
	var (
		flyPath   string
		atcServer *ghttp.Server
	)

	BeforeEach(func() {
		var err error

		flyPath, err = gexec.Build("github.com/concourse/fly")
		Ω(err).ShouldNot(HaveOccurred())
	})

	Describe("configure", func() {
		var (
			config atc.Config
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()

			os.Setenv("ATC_URL", atcServer.URL())

			config = atc.Config{
				Groups: atc.GroupConfigs{
					{
						Name:      "some-group",
						Jobs:      []string{"job-1", "job-2"},
						Resources: []string{"resource-1", "resource-2"},
					},
					{
						Name:      "some-other-group",
						Jobs:      []string{"job-3", "job-4"},
						Resources: []string{"resource-6", "resource-4"},
					},
				},

				Jobs: atc.JobConfigs{
					{
						Name: "some-orphaned-job",
					},
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/config"),
					ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigIDHeader: {"42"}}),
				),
			)
		})

		It("prints the config as yaml to stdout", func() {
			flyCmd := exec.Command(flyPath, "checklist")

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))

			Ω(string(sess.Out.Contents())).Should(Equal(fmt.Sprintf(
				`#- some-group
job-1: concourse.check %s   job-1
job-2: concourse.check %s   job-2

#- some-other-group
job-3: concourse.check %s   job-3
job-4: concourse.check %s   job-4

#- misc
some-orphaned-job: concourse.check %s   some-orphaned-job

`, atcServer.URL(), atcServer.URL(), atcServer.URL(), atcServer.URL(), atcServer.URL())))
		})
	})
})

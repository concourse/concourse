package integration_test

import (
	"os/exec"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	var (
		atcServer *ghttp.Server
	)

	Describe("workers", func() {
		var (
			args []string

			sess *gexec.Session
		)

		BeforeEach(func() {
			args = []string{}
			atcServer = ghttp.NewServer()
		})

		JustBeforeEach(func() {
			var err error

			flyCmd := exec.Command(flyPath, append([]string{"-t", atcServer.URL(), "workers"}, args...)...)

			sess, err = gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when workers are returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWithJSONEncoded(200, []atc.Worker{
							{
								Name:             "worker-2",
								GardenAddr:       "1.2.3.4:7777",
								ActiveContainers: 0,
								Platform:         "platform2",
								Tags:             []string{"tag2", "tag3"},
								ResourceTypes: []atc.WorkerResourceType{
									{Type: "resource-1", Image: "/images/resource-1"},
								},
							},
							{
								Name:             "worker-1",
								GardenAddr:       "2.2.3.4:7777",
								BaggageclaimURL:  "http://2.2.3.4:7788",
								ActiveContainers: 1,
								Platform:         "platform1",
								Tags:             []string{"tag1"},
								ResourceTypes: []atc.WorkerResourceType{
									{Type: "resource-1", Image: "/images/resource-1"},
									{Type: "resource-2", Image: "/images/resource-2"},
								},
							},
							{
								Name:             "worker-3",
								GardenAddr:       "3.2.3.4:7777",
								ActiveContainers: 10,
								Platform:         "platform3",
								Tags:             []string{},
							},
						}),
					),
				)
			})

			It("lists them to the user, ordered by name", func() {
				Eventually(sess).Should(gbytes.Say("name      containers  platform   tags      \n"))
				Eventually(sess).Should(gbytes.Say("worker-1  1           platform1  tag1      \n"))
				Eventually(sess).Should(gbytes.Say("worker-2  0           platform2  tag2, tag3\n"))
				Eventually(sess).Should(gbytes.Say("worker-3  10          platform3  none      \n"))
				Eventually(sess).Should(gexec.Exit(0))
			})

			Context("when --details is given", func() {
				BeforeEach(func() {
					args = append(args, "--details")
				})

				It("lists them to the user, ordered by name", func() {
					Eventually(sess).Should(gbytes.Say("name      containers  platform   tags        garden address  baggageclaim url     resource types"))
					Eventually(sess).Should(gbytes.Say(`worker-1  1           platform1  tag1        2.2.3.4:7777    http://2.2.3.4:7788  resource-1, resource-2`))
					Eventually(sess).Should(gbytes.Say(`worker-2  0           platform2  tag2, tag3  1.2.3.4:7777    none                 resource-1`))
					Eventually(sess).Should(gbytes.Say(`worker-3  10          platform3  none        3.2.3.4:7777    none                 none`))
					Eventually(sess).Should(gexec.Exit(0))
				})
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				Eventually(sess.Err).Should(gbytes.Say("unexpected server error"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("and the api returns an unexpected status code", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/workers"),
						ghttp.RespondWith(402, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				Eventually(sess.Err).Should(gbytes.Say("unexpected response code: 402"))
				Eventually(sess).Should(gexec.Exit(1))
			})
		})
	})
})

package integration_test

import (
	"net/http"
	"os/exec"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/ui"
	"github.com/fatih/color"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fly CLI", func() {
	var (
		atcServer *ghttp.Server
	)

	Describe("builds", func() {
		var (
			session            *gexec.Session
			cmdArgs            []string
			expectedURL        string
			queryParams        string
			returnedStatusCode int
			returnedBuilds     []atc.Build
			expectedHeaders    ui.TableRow
		)

		BeforeEach(func() {
			atcServer = ghttp.NewServer()
			cmdArgs = []string{"-t", atcServer.URL(), "builds"}

			expectedHeaders = ui.TableRow{
				{Contents: "id", Color: color.New(color.Bold)},
				{Contents: "pipeline/job#build", Color: color.New(color.Bold)},
				{Contents: "status", Color: color.New(color.Bold)},
				{Contents: "start-UTC", Color: color.New(color.Bold)},
				{Contents: "end-UTC", Color: color.New(color.Bold)},
				{Contents: "duration", Color: color.New(color.Bold)},
			}
		})

		JustBeforeEach(func() {
			var err error
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, queryParams),
					ghttp.RespondWithJSONEncoded(returnedStatusCode, returnedBuilds),
				),
			)
			cmd := exec.Command(flyPath, cmdArgs...)
			session, err = gexec.Start(cmd, nil, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("with no arguments", func() {
			var buildStillRunningTime int64

			BeforeEach(func() {
				buildStillRunningTime = time.Date(2015, time.November, 21, 10, 30, 15, 0, time.UTC).Unix()
				expectedURL = "/api/v1/builds"
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           2,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "62",
						Status:       "started",
						StartTime:    buildStillRunningTime,
						EndTime:      0,
					},
					{
						ID:           3,
						PipelineName: "some-other-pipeline",
						JobName:      "some-other-job",
						Name:         "63",
						Status:       "pending",
						StartTime:    time.Date(2015, time.December, 1, 1, 20, 15, 0, time.UTC).Unix(),
						EndTime:      time.Date(2015, time.December, 1, 2, 35, 15, 0, time.UTC).Unix(),
					},
					{
						ID:           1000001,
						PipelineName: "",
						JobName:      "",
						Name:         "",
						Status:       "errored",
						StartTime:    time.Date(2015, time.July, 4, 12, 00, 15, 0, time.UTC).Unix(),
						EndTime:      time.Date(2015, time.July, 4, 14, 45, 15, 0, time.UTC).Unix(),
					},
					{
						ID:           39,
						PipelineName: "",
						JobName:      "",
						Name:         "",
						Status:       "pending",
						StartTime:    0,
						EndTime:      0,
					},
				}
			})

			It("returns all the builds", func() {
				buildStillRunningDuration := time.Duration(time.Now().Unix()-buildStillRunningTime) * time.Second

				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "2"},
							{Contents: "some-pipeline/some-job#62"},
							{Contents: "started"},
							{Contents: "2015-11-21@10:30:15"},
							{Contents: "n/a"},
							{Contents: buildStillRunningDuration.String() + "+"}},
						{
							{Contents: "3"},
							{Contents: "some-other-pipeline/some-other-job#63"},
							{Contents: "pending"},
							{Contents: "2015-12-1@01:20:15"},
							{Contents: "2015-12-1@02:35:15"},
							{Contents: "1h15m0s"},
						},
						{
							{Contents: "1000001"},
							{Contents: "n/a"},
							{Contents: "errored"},
							{Contents: "2015-7-4@12:00:15"},
							{Contents: "2015-7-4@14:45:15"},
							{Contents: "2h45m0s"},
						},
						{
							{Contents: "39"},
							{Contents: "n/a"},
							{Contents: "pending"},
							{Contents: "n/a"},
							{Contents: "n/a"},
							{Contents: "n/a"},
						},
					},
				}))

				Eventually(session).Should(gexec.Exit(0))
			})

			Context("when the api returns an error", func() {
				BeforeEach(func() {
					returnedStatusCode = http.StatusInternalServerError
				})

				It("writes an error message to stderr", func() {
					Eventually(session.Err).Should(gbytes.Say("Unexpected Response"))
					Eventually(session).Should(gexec.Exit(1))
				})
			})
		})

		Context("when passing the limit argument", func() {
			BeforeEach(func() {
				cmdArgs = append(cmdArgs, "-c")
				cmdArgs = append(cmdArgs, "1")

				expectedURL = "/api/v1/builds"
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           39,
						PipelineName: "",
						JobName:      "",
						Name:         "",
						Status:       "pending",
						StartTime:    0,
						EndTime:      0,
					},
					{
						ID:           80,
						PipelineName: "",
						JobName:      "",
						Name:         "",
						Status:       "pending",
						StartTime:    0,
						EndTime:      0,
					},
				}
			})

			It("limits the number of returned builds", func() {
				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "39"},
							{Contents: "n/a"},
							{Contents: "pending"},
							{Contents: "n/a"},
							{Contents: "n/a"},
							{Contents: "n/a"},
						},
					},
				}))

				Eventually(session.Out).ShouldNot(PrintTable(ui.Table{
					Data: []ui.TableRow{
						{
							{Contents: "80"},
							{Contents: "n/a"},
							{Contents: "pending"},
							{Contents: "n/a"},
							{Contents: "n/a"},
							{Contents: "n/a"},
						},
					},
				}))

				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("when passing the job argument", func() {
			BeforeEach(func() {
				cmdArgs = append(cmdArgs, "-j")
				cmdArgs = append(cmdArgs, "some-pipeline/some-job")

				expectedURL = "/api/v1/pipelines/some-pipeline/jobs/some-job/builds"
				queryParams = "limit=50"
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           3,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "63",
						Status:       "succeeded",
						StartTime:    time.Date(2015, time.December, 1, 1, 20, 15, 0, time.UTC).Unix(),
						EndTime:      time.Date(2015, time.December, 1, 2, 35, 15, 0, time.UTC).Unix(),
					},
				}
			})

			It("returns the builds correctly", func() {
				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "3"},
							{Contents: "some-pipeline/some-job#63"},
							{Contents: "succeeded"},
							{Contents: "2015-12-1@01:20:15"},
							{Contents: "2015-12-1@02:35:15"},
							{Contents: "1h15m0s"},
						},
					},
				}))
				Eventually(session).Should(gexec.Exit(0))
			})

			Context("when the api returns an error", func() {
				BeforeEach(func() {
					returnedStatusCode = http.StatusInternalServerError
				})

				It("writes an error message to stderr", func() {
					Eventually(session.Err).Should(gbytes.Say("Unexpected Response"))
					Eventually(session).Should(gexec.Exit(1))
				})
			})

			Context("when the api returns a not found", func() {
				BeforeEach(func() {
					returnedStatusCode = http.StatusNotFound
				})

				It("writes an error message to stderr", func() {
					Eventually(session.Err).Should(gbytes.Say("pipleline/job not found"))
					Eventually(session).Should(gexec.Exit(1))
				})
			})

			Context("and the count argument", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "-j")
					cmdArgs = append(cmdArgs, "some-pipeline/some-job")
					cmdArgs = append(cmdArgs, "-c")
					cmdArgs = append(cmdArgs, "98")

					expectedURL = "/api/v1/pipelines/some-pipeline/jobs/some-job/builds"
					queryParams = "limit=98"
					returnedStatusCode = http.StatusOK
					returnedBuilds = []atc.Build{
						{
							ID:           3,
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Name:         "63",
							Status:       "succeeded",
							StartTime:    time.Date(2015, time.December, 1, 1, 20, 15, 0, time.UTC).Unix(),
							EndTime:      time.Date(2015, time.December, 1, 2, 35, 15, 0, time.UTC).Unix(),
						},
					}
				})

				It("returns the builds correctly", func() {
					Eventually(session.Out).Should(PrintTable(ui.Table{
						Headers: expectedHeaders,
						Data: []ui.TableRow{
							{
								{Contents: "3"},
								{Contents: "some-pipeline/some-job#63"},
								{Contents: "succeeded"},
								{Contents: "2015-12-1@01:20:15"},
								{Contents: "2015-12-1@02:35:15"},
								{Contents: "1h15m0s"},
							},
						},
					}))
					Eventually(session).Should(gexec.Exit(0))
				})
			})

		})
	})
})

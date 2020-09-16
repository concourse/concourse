package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const timeDateLayout = "2006-01-02@15:04:05-0700"
const timeLayout = "2006-01-02 15:04:05"

var _ = Describe("Fly CLI", func() {
	var (
		runningBuildStartTime   time.Time
		pendingBuildStartTime   time.Time
		pendingBuildEndTime     time.Time
		erroredBuildStartTime   time.Time
		erroredBuildEndTime     time.Time
		succeededBuildStartTime time.Time
		succeededBuildEndTime   time.Time
		zeroTime                time.Time
		abortedBuildEndTime     time.Time
	)

	BeforeEach(func() {
		runningBuildStartTime = time.Date(2015, time.November, 21, 10, 30, 15, 0, time.UTC)
		pendingBuildStartTime = time.Date(2015, time.December, 1, 1, 20, 15, 0, time.UTC)
		pendingBuildEndTime = time.Date(2015, time.December, 1, 2, 35, 15, 0, time.UTC)
		erroredBuildStartTime = time.Date(2015, time.July, 4, 12, 00, 15, 0, time.UTC)
		erroredBuildEndTime = time.Date(2015, time.July, 4, 14, 45, 15, 0, time.UTC)
		succeededBuildStartTime = time.Date(2015, time.December, 1, 1, 20, 15, 0, time.UTC)
		succeededBuildEndTime = time.Date(2015, time.December, 1, 2, 35, 15, 0, time.UTC)
		zeroTime = time.Unix(0, 0)
		abortedBuildEndTime = time.Date(2015, time.July, 4, 14, 45, 15, 0, time.UTC)
	})

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
			cmdArgs = []string{"-t", targetName, "builds"}

			expectedHeaders = ui.TableRow{
				{Contents: "id", Color: color.New(color.Bold)},
				{Contents: "pipeline/job", Color: color.New(color.Bold)},
				{Contents: "build", Color: color.New(color.Bold)},
				{Contents: "status", Color: color.New(color.Bold)},
				{Contents: "start", Color: color.New(color.Bold)},
				{Contents: "end", Color: color.New(color.Bold)},
				{Contents: "duration", Color: color.New(color.Bold)},
				{Contents: "team", Color: color.New(color.Bold)},
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
			BeforeEach(func() {
				expectedURL = "/api/v1/builds"
				queryParams = "limit=50"

				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           2,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "62",
						Status:       "started",
						StartTime:    runningBuildStartTime.Unix(),
						EndTime:      0,
						TeamName:     "team1",
					},
					{
						ID:           3,
						PipelineName: "some-other-pipeline",
						JobName:      "some-other-job",
						Name:         "63",
						Status:       "pending",
						StartTime:    pendingBuildStartTime.Unix(),
						EndTime:      pendingBuildEndTime.Unix(),
						TeamName:     "team1",
					},
					{
						ID:           1000001,
						PipelineName: "",
						JobName:      "",
						Name:         "",
						Status:       "errored",
						StartTime:    erroredBuildStartTime.Unix(),
						EndTime:      erroredBuildEndTime.Unix(),
						TeamName:     "team1",
					},
					{
						ID:           1002,
						PipelineName: "",
						JobName:      "",
						Name:         "",
						Status:       "aborted",
						StartTime:    zeroTime.Unix(),
						EndTime:      abortedBuildEndTime.Unix(),
						TeamName:     "team1",
					},
					{
						ID:           39,
						PipelineName: "",
						JobName:      "",
						Name:         "",
						Status:       "pending",
						StartTime:    0,
						EndTime:      0,
						TeamName:     "team1",
					},
				}
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--json")
				})

				It("prints response in json as stdout", func() {
					Eventually(session).Should(gexec.Exit(0))
					Expect(session.Out.Contents()).To(MatchJSON(`[
              {
                "id": 2,
                "team_name": "team1",
                "name": "62",
                "status": "started",
                "job_name": "some-job",
                "api_url": "",
                "pipeline_name": "some-pipeline",
                "start_time": 1448101815
              },
              {
                "id": 3,
                "team_name": "team1",
                "name": "63",
                "status": "pending",
                "job_name": "some-other-job",
                "api_url": "",
                "pipeline_name": "some-other-pipeline",
                "start_time": 1448932815,
                "end_time": 1448937315
              },
              {
                "id": 1000001,
                "team_name": "team1",
                "name": "",
                "status": "errored",
                "api_url": "",
                "start_time": 1436011215,
                "end_time": 1436021115
              },
              {
                "id": 1002,
                "team_name": "team1",
                "name": "",
                "status": "aborted",
                "api_url": "",
                "end_time": 1436021115
              },
              {
                "id": 39,
                "team_name": "team1",
                "name": "",
                "status": "pending",
                "api_url": ""
              }
            ]`))
				})
			})

			It("returns all the builds", func() {
				runningBuildDuration := time.Duration(time.Now().Unix()-runningBuildStartTime.Unix()) * time.Second

				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "2"},
							{Contents: "some-pipeline/some-job"},
							{Contents: "62"},
							{Contents: "started"},
							{Contents: runningBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: "n/a"},
							{
								Contents: TableDurationWithDelta{
									Duration: runningBuildDuration,
									Delta:    2 * time.Second,
									Suffix:   "+",
								}.String(),
							},
							{Contents: "team1"},
						},
						{
							{Contents: "3"},
							{Contents: "some-other-pipeline/some-other-job"},
							{Contents: "63"},
							{Contents: "pending"},
							{Contents: pendingBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: pendingBuildEndTime.Local().Format(timeDateLayout)},
							{Contents: "1h15m0s"},
							{Contents: "team1"},
						},
						{
							{Contents: "1000001"},
							{Contents: "one-off"},
							{Contents: "n/a"},
							{Contents: "errored"},
							{Contents: erroredBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: erroredBuildEndTime.Local().Format(timeDateLayout)},
							{Contents: "2h45m0s"},
							{Contents: "team1"},
						},
						{
							{Contents: "1002"},
							{Contents: "one-off"},
							{Contents: "n/a"},
							{Contents: "aborted"},
							{Contents: "n/a"},
							{Contents: abortedBuildEndTime.Local().Format(timeDateLayout)},
							{Contents: "n/a"},
							{Contents: "team1"},
						},
						{
							{Contents: "39"},
							{Contents: "one-off"},
							{Contents: "n/a"},
							{Contents: "pending"},
							{Contents: "n/a"},
							{Contents: "n/a"},
							{Contents: "n/a"},
							{Contents: "team1"},
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

		Context("when validating parameters", func() {
			Context("when specifying --all-teams and --team", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--all-teams",
						"--team", "blah")
				})

				It("instructs the user to use either --all-teams or --team", func() {
					Eventually(session.Err).Should(gbytes.Say("Cannot specify both --all-teams and --team"))
					Eventually(session).Should(gexec.Exit(1))

				})
			})

			Context("when specifying --all-teams and --current-team", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--all-teams",
						"--current-team")
				})

				It("instructs the user to not mix them together", func() {
					Eventually(session.Err).Should(gbytes.Say("Cannot specify both --all-teams and --current-team"))
					Eventually(session).Should(gexec.Exit(1))

				})
			})

			Context("when specifying --pipeline and --job", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "-j", "some-pipeline/some-job",
						"-p", "some-other-pipeline")
				})

				It("instructs the user to not mix them together", func() {
					Eventually(session.Err).Should(gbytes.Say("Cannot specify both --pipeline and --job"))
					Eventually(session).Should(gexec.Exit(1))
				})
			})
		})

		Context("when passing the limit argument", func() {
			BeforeEach(func() {
				cmdArgs = append(cmdArgs, "-c")
				cmdArgs = append(cmdArgs, "1")

				expectedURL = "/api/v1/builds"
				queryParams = "limit=1"

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
				}
			})

			It("limits the number of returned builds", func() {
				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "39"},
							{Contents: "one-off"},
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
							{Contents: "one-off"},
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

				expectedURL = "/api/v1/teams/main/pipelines/some-pipeline/jobs/some-job/builds"
				queryParams = "limit=50"
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           3,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "63",
						Status:       "succeeded",
						StartTime:    succeededBuildStartTime.Unix(),
						EndTime:      succeededBuildEndTime.Unix(),
					},
				}
			})

			It("returns the builds correctly", func() {
				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "3"},
							{Contents: "some-pipeline/some-job"},
							{Contents: "63"},
							{Contents: "succeeded"},
							{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
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
					Eventually(session.Err).Should(gbytes.Say("pipeline/job not found"))
					Eventually(session).Should(gexec.Exit(1))
				})
			})

			Context("and time range", func() {
				BeforeEach(func() {
					since := time.Date(2020, 11, 1, 0, 0, 0, 0, time.Now().Location())
					until := time.Date(2020, 11, 2, 0, 0, 0, 0, time.Now().Location())

					cmdArgs = append(cmdArgs, "-j")
					cmdArgs = append(cmdArgs, "some-pipeline/some-job")
					cmdArgs = append(cmdArgs, "--since", since.Format(timeLayout))
					cmdArgs = append(cmdArgs, "--until", until.Format(timeLayout))

					queryParams = fmt.Sprintf("limit=50&from=%d&to=%d&timestamps=true", since.Unix(), until.Unix())
				})

				It("returns the builds correctly", func() {
					Eventually(session).Should(gexec.Exit(0))
				})
			})

			Context("and the count argument", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "-j")
					cmdArgs = append(cmdArgs, "some-pipeline/some-job")
					cmdArgs = append(cmdArgs, "-c")
					cmdArgs = append(cmdArgs, "98")

					queryParams = "limit=98"
					returnedStatusCode = http.StatusOK
					returnedBuilds = []atc.Build{
						{
							ID:           3,
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Name:         "63",
							Status:       "succeeded",
							StartTime:    succeededBuildStartTime.Unix(),
							EndTime:      succeededBuildEndTime.Unix(),
						},
					}
				})

				It("returns the builds correctly", func() {
					Eventually(session.Out).Should(PrintTable(ui.Table{
						Headers: expectedHeaders,
						Data: []ui.TableRow{
							{
								{Contents: "3"},
								{Contents: "some-pipeline/some-job"},
								{Contents: "63"},
								{Contents: "succeeded"},
								{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
								{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
								{Contents: "1h15m0s"},
							},
						},
					}))
					Eventually(session).Should(gexec.Exit(0))
				})
			})
		})

		Context("when passing the current-team argument", func() {
			BeforeEach(func() {
				cmdArgs = append(cmdArgs, "--current-team")

				expectedURL = "/api/v1/teams/main/builds"
				queryParams = "limit=50"
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           3,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "63",
						Status:       "succeeded",
						StartTime:    succeededBuildStartTime.Unix(),
						EndTime:      succeededBuildEndTime.Unix(),
					},
				}
			})

			It("returns the builds correctly", func() {
				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "3"},
							{Contents: "some-pipeline/some-job"},
							{Contents: "63"},
							{Contents: "succeeded"},
							{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
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

			Context("and the count argument", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "-c")
					cmdArgs = append(cmdArgs, "98")

					queryParams = "limit=98"
					returnedStatusCode = http.StatusOK
					returnedBuilds = []atc.Build{
						{
							ID:           3,
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Name:         "63",
							Status:       "succeeded",
							StartTime:    succeededBuildStartTime.Unix(),
							EndTime:      succeededBuildEndTime.Unix(),
						},
					}
				})

				It("returns the builds correctly", func() {
					Eventually(session.Out).Should(PrintTable(ui.Table{
						Headers: expectedHeaders,
						Data: []ui.TableRow{
							{
								{Contents: "3"},
								{Contents: "some-pipeline/some-job"},
								{Contents: "63"},
								{Contents: "succeeded"},
								{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
								{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
								{Contents: "1h15m0s"},
							},
						},
					}))
					Eventually(session).Should(gexec.Exit(0))
				})
			})
		})

		Context("when passing teams argument", func() {

			Context("when passing one team filter", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--team", "team1")

					expectedURL = "/api/v1/teams/team1/builds"
					queryParams = "limit=50"
					returnedStatusCode = http.StatusOK
					returnedBuilds = []atc.Build{
						{
							ID:           3,
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Name:         "63",
							Status:       "succeeded",
							StartTime:    succeededBuildStartTime.Unix(),
							EndTime:      succeededBuildEndTime.Unix(),
							TeamName:     "team1",
						},
					}
				})

				It("returns the builds correctly", func() {
					Eventually(session.Out).Should(PrintTable(ui.Table{
						Headers: expectedHeaders,
						Data: []ui.TableRow{
							{
								{Contents: "3"},
								{Contents: "some-pipeline/some-job"},
								{Contents: "63"},
								{Contents: "succeeded"},
								{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
								{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
								{Contents: "1h15m0s"},
								{Contents: "team1"},
							},
						},
					}))
					Eventually(session).Should(gexec.Exit(0))
				})

			})

			Context("when passing multiple team filters", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "--team", "team1",
						"--team", "team2")

					expectedURL = "/api/v1/teams/team1/builds"
					queryParams = "limit=50"
					returnedStatusCode = http.StatusOK
					returnedBuilds = []atc.Build{
						{
							ID:           3,
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Name:         "63",
							Status:       "succeeded",
							StartTime:    succeededBuildStartTime.Unix(),
							EndTime:      succeededBuildEndTime.Unix(),
							TeamName:     "team1",
						},
					}
				})

				JustBeforeEach(func() {
					expectedURL = "/api/v1/teams/team2/builds"
					queryParams = "limit=50"
					returnedStatusCode = http.StatusOK
					returnedBuilds = []atc.Build{
						{
							ID:           4,
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Name:         "63",
							Status:       "succeeded",
							StartTime:    succeededBuildStartTime.Unix(),
							EndTime:      succeededBuildEndTime.Unix(),
							TeamName:     "team2",
						},
					}

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", expectedURL, queryParams),
							ghttp.RespondWithJSONEncoded(returnedStatusCode, returnedBuilds),
						),
					)
				})

				It("returns the builds correctly", func() {
					Eventually(session.Out).Should(PrintTable(ui.Table{
						Headers: expectedHeaders,
						Data: []ui.TableRow{
							{
								{Contents: "3"},
								{Contents: "some-pipeline/some-job"},
								{Contents: "63"},
								{Contents: "succeeded"},
								{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
								{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
								{Contents: "1h15m0s"},
								{Contents: "team1"},
							},

							{
								{Contents: "4"},
								{Contents: "some-pipeline/some-job"},
								{Contents: "63"},
								{Contents: "succeeded"},
								{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
								{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
								{Contents: "1h15m0s"},
								{Contents: "team2"},
							},
						},
					}))
					Eventually(session).Should(gexec.Exit(0))
				})

			})
		})

		Context("when passing all-teams argument", func() {

			BeforeEach(func() {
				expectedURL = "/api/v1/teams"
				returnedStatusCode = http.StatusOK
				returnedTeams := []atc.Team{
					{
						ID:   1,
						Name: "team1",
					},
					{
						ID:   1,
						Name: "team2",
					},
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(returnedStatusCode, returnedTeams),
					),
				)
				cmdArgs = append(cmdArgs, "--all-teams")

				expectedURL = "/api/v1/teams/team1/builds"
				queryParams = "limit=50"
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           3,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "63",
						Status:       "succeeded",
						StartTime:    succeededBuildStartTime.Unix(),
						EndTime:      succeededBuildEndTime.Unix(),
						TeamName:     "team1",
					},
				}
			})

			JustBeforeEach(func() {
				expectedURL = "/api/v1/teams/team2/builds"
				queryParams = "limit=50"
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           4,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "63",
						Status:       "succeeded",
						StartTime:    succeededBuildStartTime.Unix(),
						EndTime:      succeededBuildEndTime.Unix(),
						TeamName:     "team2",
					},
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(returnedStatusCode, returnedBuilds),
					),
				)
			})

			It("returns the builds correctly", func() {
				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "3"},
							{Contents: "some-pipeline/some-job"},
							{Contents: "63"},
							{Contents: "succeeded"},
							{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
							{Contents: "1h15m0s"},
							{Contents: "team1"},
						},
						{
							{Contents: "4"},
							{Contents: "some-pipeline/some-job"},
							{Contents: "63"},
							{Contents: "succeeded"},
							{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
							{Contents: "1h15m0s"},
							{Contents: "team2"},
						},
					},
				}))
				Eventually(session).Should(gexec.Exit(0))
			})

		})

		Context("when passing a time range", func() {
			var (
				since time.Time
				until time.Time
			)

			BeforeEach(func() {
				since = time.Date(2020, 11, 1, 0, 0, 0, 0, time.Now().Location())
				until = time.Date(2020, 11, 2, 0, 0, 0, 0, time.Now().Location())

				expectedURL = "/api/v1/builds"
				queryParams = fmt.Sprintf("limit=50&from=%d&to=%d&timestamps=true", since.Unix(), until.Unix())
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           3,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "63",
						Status:       "succeeded",
						StartTime:    succeededBuildStartTime.Unix(),
						EndTime:      succeededBuildEndTime.Unix(),
						TeamName:     "team1",
					},
				}

				cmdArgs = append(cmdArgs, "--since", since.Format(timeLayout))
				cmdArgs = append(cmdArgs, "--until", until.Format(timeLayout))
			})

			It("returns the correct builds", func() {
				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "3"},
							{Contents: "some-pipeline/some-job"},
							{Contents: "63"},
							{Contents: "succeeded"},
							{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
							{Contents: "1h15m0s"},
							{Contents: "team1"},
						},
					},
				}))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("when passing the pipeline argument", func() {
			BeforeEach(func() {
				cmdArgs = append(cmdArgs, "-p")
				cmdArgs = append(cmdArgs, "some-pipeline")

				expectedURL = "/api/v1/teams/main/pipelines/some-pipeline/builds"
				queryParams = "limit=50"
				returnedStatusCode = http.StatusOK
				returnedBuilds = []atc.Build{
					{
						ID:           3,
						PipelineName: "some-pipeline",
						JobName:      "some-job",
						Name:         "63",
						Status:       "succeeded",
						StartTime:    succeededBuildStartTime.Unix(),
						EndTime:      succeededBuildEndTime.Unix(),
					},
				}
			})

			It("returns the builds correctly", func() {
				Eventually(session.Out).Should(PrintTable(ui.Table{
					Headers: expectedHeaders,
					Data: []ui.TableRow{
						{
							{Contents: "3"},
							{Contents: "some-pipeline/some-job"},
							{Contents: "63"},
							{Contents: "succeeded"},
							{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
							{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
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
					Eventually(session.Err).Should(gbytes.Say("pipeline not found"))
					Eventually(session).Should(gexec.Exit(1))
				})
			})

			Context("and the count argument", func() {
				BeforeEach(func() {
					cmdArgs = append(cmdArgs, "-c")
					cmdArgs = append(cmdArgs, "98")

					queryParams = "limit=98"
					returnedStatusCode = http.StatusOK
					returnedBuilds = []atc.Build{
						{
							ID:           3,
							PipelineName: "some-pipeline",
							JobName:      "some-job",
							Name:         "63",
							Status:       "succeeded",
							StartTime:    succeededBuildStartTime.Unix(),
							EndTime:      succeededBuildEndTime.Unix(),
						},
					}
				})

				It("returns the builds correctly", func() {
					Eventually(session.Out).Should(PrintTable(ui.Table{
						Headers: expectedHeaders,
						Data: []ui.TableRow{
							{
								{Contents: "3"},
								{Contents: "some-pipeline/some-job"},
								{Contents: "63"},
								{Contents: "succeeded"},
								{Contents: succeededBuildStartTime.Local().Format(timeDateLayout)},
								{Contents: succeededBuildEndTime.Local().Format(timeDateLayout)},
								{Contents: "1h15m0s"},
							},
						},
					}))
					Eventually(session).Should(gexec.Exit(0))
				})
			})

			Context("and time range", func() {
				BeforeEach(func() {
					since := time.Date(2020, 11, 1, 0, 0, 0, 0, time.Now().Location())
					until := time.Date(2020, 11, 2, 0, 0, 0, 0, time.Now().Location())

					cmdArgs = append(cmdArgs, "-p")
					cmdArgs = append(cmdArgs, "some-pipeline")
					cmdArgs = append(cmdArgs, "--since", since.Format(timeLayout))
					cmdArgs = append(cmdArgs, "--until", until.Format(timeLayout))

					queryParams = fmt.Sprintf("limit=50&from=%d&to=%d&timestamps=true", since.Unix(), until.Unix())
				})

				It("returns the builds correctly", func() {
					Eventually(session).Should(gexec.Exit(0))
				})
			})
		})
	})
})

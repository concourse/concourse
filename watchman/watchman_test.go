package watchman_test

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/router"
	"github.com/winston-ci/prole/api/builds"
	"github.com/winston-ci/prole/routes"

	"github.com/winston-ci/winston/builder/fakebuilder"
	"github.com/winston-ci/winston/config"
	. "github.com/winston-ci/winston/watchman"
)

var _ = Describe("Watchman", func() {
	var proleServer *ghttp.Server

	var builder *fakebuilder.Builder
	var watchman Watchman

	var job config.Job
	var input config.Input
	var interval time.Duration

	var stop chan<- struct{}

	BeforeEach(func() {
		proleServer = ghttp.NewServer()
		proleServer.AllowUnhandledRequests = true

		builder = fakebuilder.New()

		watchman = NewWatchman(
			builder,
			router.NewRequestGenerator(proleServer.URL(), routes.Routes),
		)

		interval = 100 * time.Millisecond

		job = config.Job{
			Name: "some-job",
			Inputs: []config.Input{
				{
					Name:   "some-input",
					Type:   "git",
					Source: config.Source("123"),
				},
			},
		}

		input = job.Inputs[0]
	})

	JustBeforeEach(func() {
		stop = watchman.Watch(job, input, interval)
	})

	AfterEach(func() {
		close(stop)
	})

	Context("when the endpoint is functioning", func() {
		var times <-chan time.Time

		var returnedSources1 []builds.Source
		var returnedSources2 []builds.Source

		BeforeEach(func() {
			returnedSources1 = []builds.Source{}
			returnedSources2 = []builds.Source{}

			timesChecked := make(chan time.Time, 2)

			times = timesChecked

			proleServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/checks"),
					ghttp.VerifyJSONRepresenting(builds.Input{
						Type:   input.Type,
						Source: builds.Source("123"),
					}),
					func(w http.ResponseWriter, r *http.Request) {
						timesChecked <- time.Now()
						ghttp.RespondWithJSONEncoded(200, returnedSources1)(w, r)
					},
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/checks"),
					func(w http.ResponseWriter, r *http.Request) {
						timesChecked <- time.Now()
						ghttp.RespondWithJSONEncoded(200, returnedSources2)(w, r)
					},
				),
			)
		})

		It("polls /checks on a specified interval", func() {
			var time1 time.Time
			var time2 time.Time

			Eventually(times).Should(Receive(&time1))
			Eventually(times).Should(Receive(&time2))

			Ω(time2.Sub(time1)).Should(BeNumerically(">=", interval/2))
		})

		Context("but checking takes a while", func() {
			BeforeEach(func() {
				proleServer.WrapHandler(0,
					func(w http.ResponseWriter, r *http.Request) {
						time.Sleep(interval)
					},
				)
			})

			It("does not count it towards the interval", func() {
				var time1 time.Time
				var time2 time.Time

				Eventually(times).Should(Receive(&time1))
				Eventually(times, 2).Should(Receive(&time2))

				Ω(time2.Sub(time1)).Should(BeNumerically(">", interval))
				Ω(time2.Sub(time1)).Should(BeNumerically("<", interval*2))
			})
		})

		Context("and it returns new sources", func() {
			var verifiedSecond chan struct{}

			BeforeEach(func() {
				verifiedSecond = make(chan struct{})

				returnedSources1 = []builds.Source{
					builds.Source(`"abc"`),
					builds.Source(`"def"`),
				}

				returnedSources2 = []builds.Source{
					builds.Source(`"ghi"`),
				}

				proleServer.WrapHandler(
					1,
					ghttp.CombineHandlers(
						ghttp.VerifyJSONRepresenting(builds.Input{
							Type:   input.Type,
							Source: builds.Source(`"def"`),
						}),
						func(w http.ResponseWriter, r *http.Request) {
							close(verifiedSecond)
						},
					),
				)
			})

			It("builds the job with the changed source", func() {
				Eventually(builder.Built).Should(ContainElement(config.Job{
					Name: "some-job",
					Inputs: []config.Input{
						{
							Name:   "some-input",
							Type:   "git",
							Source: config.Source(`"abc"`),
						},
					},
				}))

				Eventually(builder.Built).Should(ContainElement(config.Job{
					Name: "some-job",
					Inputs: []config.Input{
						{
							Name:   "some-input",
							Type:   "git",
							Source: config.Source(`"def"`),
						},
					},
				}))

				Eventually(builder.Built).Should(ContainElement(config.Job{
					Name: "some-job",
					Inputs: []config.Input{
						{
							Name:   "some-input",
							Type:   "git",
							Source: config.Source(`"ghi"`),
						},
					},
				}))
			})

			It("watches from the most recent source", func() {
				Eventually(verifiedSecond).Should(BeClosed())
			})
		})
	})
})

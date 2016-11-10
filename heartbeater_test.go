package tsa_test

import (
	"encoding/json"
	"net/http"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/garden"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	. "github.com/concourse/tsa"
	"github.com/concourse/tsa/tsafakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/rata"
)

var _ = Describe("Heartbeater", func() {
	type registration struct {
		worker atc.Worker
		ttl    time.Duration
	}

	var (
		logger lager.Logger

		addrToRegister string
		fakeClock      *fakeclock.FakeClock
		interval       time.Duration
		cprInterval    time.Duration
		resourceTypes  []atc.WorkerResourceType

		expectedWorker     atc.Worker
		fakeTokenGenerator *tsafakes.FakeTokenGenerator
		fakeGardenClient   *gfakes.FakeClient
		fakeATC            *ghttp.Server

		heartbeater ifrit.Process

		verifyRegister  http.HandlerFunc
		verifyHeartbeat http.HandlerFunc

		registrations <-chan registration
		heartbeats    <-chan registration
		clientWriter  *gbytes.Buffer

		worker atc.Worker
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		addrToRegister = "1.2.3.4:7777"
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		interval = time.Second
		cprInterval = 100 * time.Millisecond
		resourceTypes = []atc.WorkerResourceType{
			{
				Type:  "git",
				Image: "/path/to/git/resource",
			},
		}

		worker = atc.Worker{
			Name:          "some-name",
			GardenAddr:    addrToRegister,
			ResourceTypes: resourceTypes,
			Platform:      "some-platform",
			Tags:          []string{"some", "tags"},
		}

		expectedWorker = atc.Worker{
			Name:             "some-name",
			GardenAddr:       addrToRegister,
			ActiveContainers: 2,
			ResourceTypes:    resourceTypes,
			Platform:         "some-platform",
			Tags:             []string{"some", "tags"},
		}

		fakeATC = ghttp.NewServer()

		registerRoute, found := atc.Routes.FindRouteByName(atc.RegisterWorker)
		Expect(found).To(BeTrue())

		registered := make(chan registration, 100)
		registrations = registered

		heartbeated := make(chan registration, 100)
		heartbeats = heartbeated

		verifyRegister = ghttp.CombineHandlers(
			ghttp.VerifyRequest(registerRoute.Method, registerRoute.Path),
			func(w http.ResponseWriter, r *http.Request) {
				var worker atc.Worker
				Expect(r.Header.Get("Authorization")).To(Equal("Bearer yo"))

				err := json.NewDecoder(r.Body).Decode(&worker)
				Expect(err).NotTo(HaveOccurred())

				ttl, err := time.ParseDuration(r.URL.Query().Get("ttl"))
				Expect(err).NotTo(HaveOccurred())

				registered <- registration{worker, ttl}
			},
		)

		verifyHeartbeat = ghttp.CombineHandlers(
			ghttp.VerifyRequest("PUT", "/api/v1/workers/some-name/heartbeat"),
			func(w http.ResponseWriter, r *http.Request) {
				var worker atc.Worker
				Expect(r.Header.Get("Authorization")).To(Equal("Bearer yo"))

				err := json.NewDecoder(r.Body).Decode(&worker)
				Expect(err).NotTo(HaveOccurred())

				ttl, err := time.ParseDuration(r.URL.Query().Get("ttl"))
				Expect(err).NotTo(HaveOccurred())

				heartbeated <- registration{worker, ttl}
			},
		)

		fakeGardenClient = new(gfakes.FakeClient)
		fakeTokenGenerator = new(tsafakes.FakeTokenGenerator)

		fakeTokenGenerator.GenerateTokenReturns("yo", nil)
		clientWriter = gbytes.NewBuffer()
	})

	JustBeforeEach(func() {
		atcEndpoint := rata.NewRequestGenerator(fakeATC.URL(), atc.Routes)
		heartbeater = ifrit.Invoke(
			NewHeartbeater(
				logger,
				fakeClock,
				interval,
				cprInterval,
				fakeGardenClient,
				atcEndpoint,
				fakeTokenGenerator,
				worker,
				clientWriter,
			),
		)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(heartbeater)
	})

	Context("when Garden returns containers", func() {
		BeforeEach(func() {
			containers := make(chan []garden.Container, 4)

			containers <- []garden.Container{
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
			}

			containers <- []garden.Container{
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
			}

			containers <- []garden.Container{
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
			}

			containers <- []garden.Container{
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
				new(gfakes.FakeContainer),
			}

			close(containers)

			fakeGardenClient.ContainersStub = func(garden.Properties) ([]garden.Container, error) {
				return <-containers, nil
			}
		})

		Context("when the ATC responds to registration requests", func() {
			BeforeEach(func() {
				fakeATC.AppendHandlers(verifyRegister, verifyHeartbeat)
			})

			It("immediately registers", func() {
				Expect(registrations).To(Receive(Equal(registration{expectedWorker, 2 * interval})))
			})

			It("heartbeats", func() {
				Expect(registrations).To(Receive())

				fakeClock.WaitForWatcherAndIncrement(interval)
				expectedWorker.ActiveContainers = 5
				Eventually(heartbeats).Should(Receive(Equal(registration{expectedWorker, 2 * interval})))
			})
		})

		Context("when the ATC doesn't respond to the first heartbeat", func() {
			BeforeEach(func() {
				fakeATC.AppendHandlers(
					verifyRegister,
					ghttp.CombineHandlers(
						verifyHeartbeat,
						func(w http.ResponseWriter, r *http.Request) { fakeATC.CloseClientConnections() },
					),
					verifyHeartbeat,
					verifyHeartbeat,
				)
			})

			It("heartbeats faster according to cprInterval", func() {
				Expect(registrations).To(Receive())

				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(heartbeats).Should(Receive())

				fakeClock.WaitForWatcherAndIncrement(cprInterval)
				expectedWorker.ActiveContainers = 4
				Eventually(heartbeats).Should(Receive(Equal(registration{expectedWorker, 2 * interval})))
			})

			It("goes back to normal after the heartbeat succeeds", func() {
				Expect(registrations).To(Receive())

				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(heartbeats).Should(Receive())

				fakeClock.WaitForWatcherAndIncrement(cprInterval)
				Eventually(heartbeats).Should(Receive())

				fakeClock.WaitForWatcherAndIncrement(cprInterval)
				Consistently(heartbeats).ShouldNot(Receive())

				fakeClock.WaitForWatcherAndIncrement(interval - cprInterval)
				expectedWorker.ActiveContainers = 3
				Eventually(heartbeats).Should(Receive(Equal(registration{expectedWorker, 2 * interval})))
			})
		})
	})
})

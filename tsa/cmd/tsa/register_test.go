package main_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/tsa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Register", func() {
	var opts tsa.RegisterOptions
	var registerCtx context.Context
	var cancel context.CancelFunc
	var registerErr chan error

	BeforeEach(func() {
		opts = tsa.RegisterOptions{
			LocalGardenNetwork: "tcp",
			LocalGardenAddr:    gardenAddr,

			LocalBaggageclaimNetwork: "tcp",
			LocalBaggageclaimAddr:    baggageclaimServer.Addr(),
		}

		registerCtx, cancel = context.WithCancel(context.Background())

		errs := make(chan error, 1)
		registerErr = errs
	})

	JustBeforeEach(func() {
		go func() {
			registerErr <- tsaClient.Register(registerCtx, opts)
			close(registerErr)
		}()
	})

	AfterEach(func() {
		cancel()
		<-registerErr
	})

	itSuccessfullyRegistersAndHeartbeats := func() {
		BeforeEach(func() {
			gardenStubs := make(chan func() ([]garden.Container, error), 4)

			gardenStubs <- func() ([]garden.Container, error) {
				return []garden.Container{
					new(gfakes.FakeContainer),
					new(gfakes.FakeContainer),
					new(gfakes.FakeContainer),
				}, nil
			}

			gardenStubs <- func() ([]garden.Container, error) {
				return []garden.Container{
					new(gfakes.FakeContainer),
					new(gfakes.FakeContainer),
				}, nil
			}

			gardenStubs <- func() ([]garden.Container, error) {
				return nil, errors.New("garden was weeded")
			}

			gardenStubs <- func() ([]garden.Container, error) {
				return []garden.Container{
					new(gfakes.FakeContainer),
				}, nil
			}

			fakeBackend.ContainersStub = func(garden.Properties) ([]garden.Container, error) {
				return (<-gardenStubs)()
			}

			baggageclaimServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/volumes"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []baggageclaim.VolumeResponse{
						{Handle: "handle-a"},
						{Handle: "handle-b"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/volumes"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []baggageclaim.VolumeResponse{
						{Handle: "handle-a"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/volumes"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []baggageclaim.VolumeResponse{
						{Handle: "handle-a"},
						{Handle: "handle-b"},
						{Handle: "handle-c"},
					}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/volumes"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, []baggageclaim.VolumeResponse{}),
				),
			)
		})

		It("forwards garden and baggageclaim API calls through the tunnel", func() {
			registration := <-registered
			addr := registration.worker.GardenAddr

			gClient := gclient.New(gconn.New("tcp", addr))

			fakeBackend.CreateReturns(new(gfakes.FakeContainer), nil)

			_, err := gClient.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBackend.CreateCallCount()).To(Equal(1))
		})

		It("continuously registers it with the ATC as long as it works", func() {
			By("initially registering")
			a := time.Now()
			registration := <-registered
			Expect(registration.ttl).To(Equal(2 * heartbeatInterval))
			expectedWorkerPayload := tsaClient.Worker
			expectedWorkerPayload.GardenAddr = registration.worker.GardenAddr
			expectedWorkerPayload.BaggageclaimURL = registration.worker.BaggageclaimURL
			expectedWorkerPayload.ActiveContainers = 3
			expectedWorkerPayload.ActiveVolumes = 2

			By("registering a forwarded garden address")
			host, port, err := net.SplitHostPort(registration.worker.GardenAddr)
			Expect(err).NotTo(HaveOccurred())
			Expect(host).To(Equal(forwardHost))
			Expect(port).NotTo(Equal("7777")) // should NOT respect bind addr

			By("registering a forwarded baggageclaim address")
			bURL, err := url.Parse(registration.worker.BaggageclaimURL)
			Expect(err).NotTo(HaveOccurred())
			host, port, err = net.SplitHostPort(bURL.Host)
			Expect(err).NotTo(HaveOccurred())
			Expect(host).To(Equal(forwardHost))
			Expect(port).NotTo(Equal("7788")) // should NOT respect bind addr

			By("heartbeating")
			b := time.Now()
			registration = <-heartbeated
			Expect(registration.ttl).To(Equal(2 * heartbeatInterval))
			expectedWorkerPayload = tsaClient.Worker
			expectedWorkerPayload.GardenAddr = registration.worker.GardenAddr
			expectedWorkerPayload.BaggageclaimURL = registration.worker.BaggageclaimURL
			expectedWorkerPayload.ActiveContainers = 2
			expectedWorkerPayload.ActiveVolumes = 1
			Expect(registration.worker).To(Equal(expectedWorkerPayload))

			By("heartbeating a forwarded garden address")
			host, port, err = net.SplitHostPort(registration.worker.GardenAddr)
			Expect(err).NotTo(HaveOccurred())
			Expect(host).To(Equal(forwardHost))
			Expect(port).NotTo(Equal("7777")) // should NOT respect bind addr

			By("heartbeating a forwarded baggageclaim address")
			bURL, err = url.Parse(registration.worker.BaggageclaimURL)
			Expect(err).NotTo(HaveOccurred())
			host, port, err = net.SplitHostPort(bURL.Host)
			Expect(err).NotTo(HaveOccurred())
			Expect(host).To(Equal(forwardHost))
			Expect(port).NotTo(Equal("7788")) // should NOT respect bind addr

			By("having heartbeated after the interval")
			Expect(b.Sub(a)).To(BeNumerically("~", heartbeatInterval, 1*time.Second))

			By("not heartbeating when garden returns an error")
			Consistently(heartbeated, 2*heartbeatInterval).ShouldNot(Receive())

			By("eventually heartbeating again once it's ok")
			c := time.Now()
			registration = <-heartbeated
			Expect(registration.ttl).To(Equal(2 * heartbeatInterval))
			expectedWorkerPayload = tsaClient.Worker
			expectedWorkerPayload.GardenAddr = registration.worker.GardenAddr
			expectedWorkerPayload.BaggageclaimURL = registration.worker.BaggageclaimURL
			expectedWorkerPayload.ActiveContainers = 1
			expectedWorkerPayload.ActiveVolumes = 0
			Expect(registration.worker).To(Equal(expectedWorkerPayload))

			By("having heartbeated after another interval passed")
			Expect(c.Sub(b)).To(BeNumerically("~", 3*heartbeatInterval, 1*time.Second))
		})

		Context("when the ATC returns a 404 for the heartbeat", func() {
			BeforeEach(func() {
				atcServer.RouteToHandler("PUT", "/api/v1/workers/some-worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
					Expect(accessFactory.Create(r, "some-action").IsAuthenticated()).To(BeTrue())
					w.WriteHeader(404)
				})
			})

			It("exits gracefully", func() {
				Expect(<-registerErr).ToNot(HaveOccurred())
			})
		})

		Context("when the client goes away", func() {
			It("stops registering", func() {
				time.Sleep(heartbeatInterval)

				cancel()
				<-registerErr

				time.Sleep(heartbeatInterval)

				// siphon off any existing registrations
			dance:
				for {
					select {
					case <-registered:
					case <-heartbeated:
					default:
						break dance
					}
				}

				Consistently(registered, 2*heartbeatInterval).ShouldNot(Receive())
				Consistently(heartbeated, 2*heartbeatInterval).ShouldNot(Receive())
			})
		})
	}

	Context("when the worker is global", func() {
		BeforeEach(func() {
			tsaClient.Worker.Team = ""
		})

		Context("when the key is globally authorized", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = globalKey
			})

			itSuccessfullyRegistersAndHeartbeats()
		})

		Context("when the key is not authorized", func() {
			BeforeEach(func() {
				_, _, badKey, _ := generateSSHKeypair()
				tsaClient.PrivateKey = badKey
			})

			It("returns ErrUnauthorized", func() {
				Expect(<-registerErr).To(Equal(tsa.ErrUnauthorized))
			})
		})
	})

	Context("when the worker is for a given team", func() {
		BeforeEach(func() {
			tsaClient.Worker.Team = "some-team"
		})

		Context("when the key is globally authorized", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = globalKey
			})

			itSuccessfullyRegistersAndHeartbeats()
		})

		Context("when the key is authorized for the same team", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = teamKey
			})

			itSuccessfullyRegistersAndHeartbeats()
		})

		Context("when the key is authorized for some other team", func() {
			BeforeEach(func() {
				tsaClient.PrivateKey = otherTeamKey
			})

			It("returns an error", func() {
				// XXX: cleaner error
				Expect(<-registerErr).To(HaveOccurred())
			})
		})

		Context("when the key is not authorized", func() {
			BeforeEach(func() {
				_, _, badKey, _ := generateSSHKeypair()
				tsaClient.PrivateKey = badKey
			})

			It("returns ErrUnauthorized", func() {
				Expect(<-registerErr).To(Equal(tsa.ErrUnauthorized))
			})
		})
	})
})

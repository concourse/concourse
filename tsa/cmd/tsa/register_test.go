package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/tsa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type registration struct {
	worker atc.Worker
	ttl    time.Duration
}

type workerState struct {
	retired bool
	landed  bool
	stalled bool
}

var _ = Describe("Register", func() {
	var opts tsa.RegisterOptions
	var registerDone <-chan struct{}
	var heartbeatEvent <-chan struct{}
	var registerCtx context.Context
	var cancel context.CancelFunc
	var registerErr chan error

	var registered chan registration
	var heartbeated chan registration
	var heartbeatResults chan workerState

	BeforeEach(func() {
		registered = make(chan registration, 100)
		heartbeated = make(chan registration, 100)
		heartbeatResults = make(chan workerState, 100)

		atcServer.RouteToHandler("POST", "/api/v1/workers", func(w http.ResponseWriter, r *http.Request) {
			var worker atc.Worker

			err := json.NewDecoder(r.Body).Decode(&worker)
			Expect(err).NotTo(HaveOccurred())

			ttl, err := time.ParseDuration(r.URL.Query().Get("ttl"))
			Expect(err).NotTo(HaveOccurred())

			registered <- registration{worker, ttl}
		})

		atcServer.RouteToHandler("PUT", "/api/v1/workers/some-worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
			var worker atc.Worker

			err := json.NewDecoder(r.Body).Decode(&worker)
			Expect(err).NotTo(HaveOccurred())

			ttl, err := time.ParseDuration(r.URL.Query().Get("ttl"))
			Expect(err).NotTo(HaveOccurred())

			heartbeated <- registration{worker, ttl}

			select {
			case res := <-heartbeatResults:
				if res.retired {
					w.WriteHeader(http.StatusNotFound)
					return
				} else if res.landed {
					worker.State = "landed"
				}
			default:
			}

			json.NewEncoder(w).Encode(worker)
		})

		reg := make(chan struct{})
		beat := make(chan struct{}, 100)

		opts = tsa.RegisterOptions{
			LocalGardenNetwork: "tcp",
			LocalGardenAddr:    gardenAddr,

			LocalBaggageclaimNetwork: "tcp",
			LocalBaggageclaimAddr:    baggageclaimServer.Addr(),

			RegisteredFunc: func() {
				close(reg)
			},

			HeartbeatedFunc: func() {
				beat <- struct{}{}
			},
		}

		registerDone = reg
		heartbeatEvent = beat

		registerCtx, cancel = context.WithCancel(context.Background())
	})

	JustBeforeEach(func() {
		errs := make(chan error, 1)
		registerErr = errs

		go func() {
			errs <- tsaClient.Register(lagerctx.NewContext(registerCtx, lagertest.NewTestLogger("test")), opts)
			close(errs)
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
				return nil, errors.New("forced to fail by tests")
			}

			gardenStubs <- func() ([]garden.Container, error) {
				return []garden.Container{
					new(gfakes.FakeContainer),
				}, nil
			}

			close(gardenStubs)

			fakeBackend.ContainersStub = func(garden.Properties) ([]garden.Container, error) {
				stub, ok := <-gardenStubs
				if ok {
					return stub()
				}

				return nil, errors.New("not stubbed enough")
			}

			baggageclaimStubs := make(chan func() ([]baggageclaim.VolumeResponse, error), 4)

			baggageclaimStubs <- func() ([]baggageclaim.VolumeResponse, error) {
				return []baggageclaim.VolumeResponse{
					{Handle: "handle-a"},
					{Handle: "handle-b"},
				}, nil
			}

			baggageclaimStubs <- func() ([]baggageclaim.VolumeResponse, error) {
				return []baggageclaim.VolumeResponse{
					{Handle: "handle-a"},
				}, nil
			}

			baggageclaimStubs <- func() ([]baggageclaim.VolumeResponse, error) {
				return []baggageclaim.VolumeResponse{
					{Handle: "handle-a"},
					{Handle: "handle-b"},
					{Handle: "handle-c"},
				}, nil
			}

			baggageclaimStubs <- func() ([]baggageclaim.VolumeResponse, error) {
				return []baggageclaim.VolumeResponse{}, nil
			}

			close(baggageclaimStubs)

			baggageclaimServer.RouteToHandler("GET", "/volumes", func(w http.ResponseWriter, r *http.Request) {
				stub, ok := <-baggageclaimStubs
				if !ok {
					w.WriteHeader(http.StatusTeapot)
					json.NewEncoder(w).Encode(struct {
						Error string `json:"error"`
					}{
						Error: "baggageclaim not stubbed enough",
					})
					return
				}

				vols, err := stub()
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "stubbed error: %s", err)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(vols)
			})
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

		It("fires the registered and heartbeated callbacks", func() {
			<-registerDone
			<-heartbeatEvent
			<-heartbeatEvent
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

		Context("when the worker has landed", func() {
			It("does not wait for connections to complete before exiting", func() {
				By("waiting for an initial registration")
				registration := <-registered

				baggageclaimServer.RouteToHandler("GET", "/slow", func(w http.ResponseWriter, r *http.Request) {
					By("sleeping during the request")
					time.Sleep(5 * time.Second)

					w.WriteHeader(http.StatusTeapot)
				})

				By("hitting a slow endpoint on " + registration.worker.BaggageclaimURL)
				stubbed := false
				client := &http.Client{
					Transport: &http.Transport{
						// disable keepalives so the connection doesn't hang around
						DisableKeepAlives: true,

						// ensure we stub out the landing after the connection is established
						Dial: func(netw, addr string) (net.Conn, error) {
							conn, err := net.Dial(netw, addr)
							if err != nil {
								return nil, err
							}

							stubbed = true
							heartbeatResults <- workerState{
								landed: true,
							}

							return conn, nil
						},
					},
				}

				_, err := client.Get(registration.worker.BaggageclaimURL + "/slow")
				Expect(err).To(HaveOccurred())

				Expect(stubbed).To(BeTrue())

				By("exiting successfully")
				Eventually(registerErr).Should(Receive(BeNil()))
			})
		})

		Context("when the worker has retired", func() {
			It("does not wait for connections to complete before exiting", func() {
				By("waiting for an initial registration")
				registration := <-registered

				baggageclaimServer.RouteToHandler("GET", "/slow", func(w http.ResponseWriter, r *http.Request) {
					By("sleeping during the request")
					time.Sleep(5 * time.Second)

					w.WriteHeader(http.StatusTeapot)
				})

				By("hitting a slow endpoint on " + registration.worker.BaggageclaimURL)
				stubbed := false
				client := &http.Client{
					Transport: &http.Transport{
						// disable keepalives so the connection doesn't hang around
						DisableKeepAlives: true,

						// ensure we stub out the retiring after the connection is established
						Dial: func(netw, addr string) (net.Conn, error) {
							conn, err := net.Dial(netw, addr)
							if err != nil {
								return nil, err
							}

							stubbed = true
							heartbeatResults <- workerState{
								retired: true,
							}

							return conn, nil
						},
					},
				}

				_, err := client.Get(registration.worker.BaggageclaimURL + "/slow")
				Expect(err).To(HaveOccurred())

				Expect(stubbed).To(BeTrue())

				By("exiting successfully")
				Eventually(registerErr).Should(Receive(BeNil()))
			})
		})

		Describe("canceling the context", func() {
			It("waits for the connections to complete before exiting", func() {
				By("waiting for an initial registration")
				registration := <-registered

				baggageclaimServer.RouteToHandler("GET", "/slow", func(w http.ResponseWriter, r *http.Request) {
					By("canceling during the request")
					cancel()

					By("sleeping during the request")
					time.Sleep(5 * time.Second)

					w.WriteHeader(http.StatusTeapot)
				})

				By("hitting a slow endpoint on " + registration.worker.BaggageclaimURL)
				client := &http.Client{
					Transport: &http.Transport{
						// disable keepalives so the connection doesn't hang around
						DisableKeepAlives: true,
					},
				}

				res, err := client.Get(registration.worker.BaggageclaimURL + "/slow")
				Expect(err).ToNot(HaveOccurred())
				Expect(res.StatusCode).To(Equal(http.StatusTeapot))

				By("exiting successfully")
				Eventually(registerErr).Should(Receive(BeNil()))
			})

			Context("with a drain timeout", func() {
				BeforeEach(func() {
					opts.ConnectionDrainTimeout = 5 * time.Second
				})

				It("breaks connections after the configured drain timeout", func() {
					By("waiting for an initial registration")
					registration := <-registered

					baggageclaimServer.RouteToHandler("GET", "/noop", func(w http.ResponseWriter, r *http.Request) {
						By("canceling during the request")
						cancel()

						w.WriteHeader(http.StatusTeapot)
					})

					By("opening a connection")
					client := &http.Client{
						Transport: &http.Transport{
							// explicitly enable keepalives, just so the test is more obvious
							// in keeping a connection open
							DisableKeepAlives: false,
						},
					}
					res, err := client.Get(registration.worker.BaggageclaimURL + "/noop")
					Expect(err).ToNot(HaveOccurred())
					Expect(res.StatusCode).To(Equal(http.StatusTeapot))

					By("waiting for connections to be idle before exiting")
					before := time.Now()
					Expect(<-registerErr).To(Equal(tsa.ErrConnectionDrainTimeout))
					Expect(time.Now().Sub(before)).To(BeNumerically("~", opts.ConnectionDrainTimeout, time.Second))
				})
			})
		})

		Context("when the ATC returns a 404 for the heartbeat", func() {
			BeforeEach(func() {
				atcServer.RouteToHandler("PUT", "/api/v1/workers/some-worker/heartbeat", func(w http.ResponseWriter, r *http.Request) {
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

			It("returns *HandshakeError", func() {
				Expect(<-registerErr).To(BeAssignableToTypeOf(&tsa.HandshakeError{}))
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
				Expect(<-registerErr).To(HaveOccurred())
			})
		})

		Context("when the key is not authorized", func() {
			BeforeEach(func() {
				_, _, badKey, _ := generateSSHKeypair()
				tsaClient.PrivateKey = badKey
			})

			It("returns *HandshakeError", func() {
				Expect(<-registerErr).To(BeAssignableToTypeOf(&tsa.HandshakeError{}))
			})
		})
	})
})

package beacon_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/tsa"
	. "github.com/concourse/worker/beacon"
	"github.com/concourse/worker/beacon/beaconfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Beacon", func() {

	var (
		beacon        Beacon
		fakeClient    *beaconfakes.FakeClient
		fakeSession   *beaconfakes.FakeSession
		fakeCloseable *beaconfakes.FakeCloseable
		fakeVolumeOne *baggageclaimfakes.FakeVolume
		fakeVolumeTwo *baggageclaimfakes.FakeVolume
	)

	BeforeEach(func() {
		fakeClient = new(beaconfakes.FakeClient)
		fakeSession = new(beaconfakes.FakeSession)
		fakeCloseable = new(beaconfakes.FakeCloseable)
		fakeVolumeOne = new(baggageclaimfakes.FakeVolume)
		fakeVolumeTwo = new(baggageclaimfakes.FakeVolume)
		fakeClient.NewSessionReturns(fakeSession, nil)
		fakeClient.DialReturns(fakeCloseable, nil)
		logger := lager.NewLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		beacon = Beacon{
			KeepAlive: true,
			Logger:    logger,
			Client:    fakeClient,
			Worker: atc.Worker{
				GardenAddr:      "1.2.3.4:7777",
				BaggageclaimURL: "wat://5.6.7.8:7788",
			},
		}
	})

	var _ = Describe("Register", func() {
		var (
			signals     chan os.Signal
			ready       chan<- struct{}
			registerErr error
			exited      chan error
		)

		JustBeforeEach(func() {
			signals = make(chan os.Signal, 1)
			ready = make(chan struct{}, 1)
		})

		AfterEach(func() {
			Expect(fakeCloseable.CloseCallCount()).To(Equal(1))
		})

		Context("when the exit channel takes time to exit", func() {
			var (
				keepAliveErr    chan error
				cancelKeepAlive chan struct{}
				wait            chan bool
			)
			BeforeEach(func() {
				keepAliveErr = make(chan error, 1)
				cancelKeepAlive = make(chan struct{}, 1)
				wait = make(chan bool, 1)
				exited = make(chan error, 1)

				fakeSession.WaitStub = func() error {
					<-wait
					signals <- syscall.SIGKILL
					return errors.New("bad-err")
				}

				fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
				go func() {
					exited <- beacon.Register(signals, make(chan struct{}, 1))
				}()

			})

			It("closes the session and waits for it to shut down", func() {
				Consistently(exited).ShouldNot(Receive()) // should be blocking on exit channel
				go func() {
					wait <- false
				}()
				Eventually(exited).Should(Receive()) // should stop blocking
				Expect(fakeSession.CloseCallCount()).To(Equal(2))
			})
		})

		Context("when exiting immediately", func() {

			JustBeforeEach(func() {
				registerErr = beacon.Register(signals, ready)
			})

			Context("when waiting on the session errors", func() {
				BeforeEach(func() {
					fakeSession.WaitReturns(errors.New("fail"))
				})
				It("returns the error", func() {
					Expect(registerErr).To(Equal(errors.New("fail")))
				})
			})

			Context("when the runner recieves a signal", func() {
				var (
					keepAliveErr    chan error
					cancelKeepAlive chan struct{}
				)
				BeforeEach(func() {
					keepAliveErr = make(chan error, 1)
					cancelKeepAlive = make(chan struct{}, 1)

					wait := make(chan bool, 1)
					fakeSession.WaitStub = func() error {
						<-wait
						return nil
					}

					fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
					go func() {
						signals <- syscall.SIGKILL
						wait <- false
					}()

				})

				It("stops the keepalive", func() {
					Eventually(cancelKeepAlive).Should(BeClosed())
				})
			})

			Context("when keeping the connection alive errors", func() {
				var (
					keepAliveErr    chan error
					cancelKeepAlive chan<- struct{}
				)

				BeforeEach(func() {
					wait := make(chan bool, 1)
					fakeSession.WaitStub = func() error {
						<-wait
						return nil
					}

					keepAliveErr = make(chan error, 1)
					cancelKeepAlive = make(chan struct{}, 1)

					fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
					go func() {
						keepAliveErr <- errors.New("keepalive fail")
					}()
				})

				It("returns the error", func() {
					Expect(registerErr).To(Equal(errors.New("keepalive fail")))
				})
			})

			// Context("when the registration mode is 'forward'", func() {
			// 	BeforeEach(func() {
			// 		beacon.RegistrationMode = Forward
			// 	})

			// 	It("Forwards the worker's Garden and Baggageclaim to TSA", func() {
			// 		By("using the forward-worker command")
			// 		Expect(fakeSession.StartCallCount()).To(Equal(1))
			// 		Expect(fakeSession.StartArgsForCall(0)).To(Equal("forward-worker --garden 0.0.0.0:7777 --baggageclaim 0.0.0.0:7788"))
			// 	})
			// })

			// Context("when the registration mode is 'direct'", func() {
			// 	BeforeEach(func() {
			// 		beacon.RegistrationMode = Direct
			// 	})

			// 	It("Registers directly with the TSA", func() {
			// 		By("using the register-worker command")
			// 		Expect(fakeSession.StartCallCount()).To(Equal(1))
			// 		Expect(fakeSession.StartArgsForCall(0)).To(Equal("register-worker"))
			// 	})
			// })

			// It("Forwards the worker's Garden and Baggageclaim to TSA by default", func() {
			// 	By("using the forward-worker command")
			// 	Expect(fakeSession.StartCallCount()).To(Equal(1))
			// 	Expect(fakeSession.StartArgsForCall(0)).To(Equal("forward-worker --garden 0.0.0.0:7777 --baggageclaim 0.0.0.0:7788"))
			// })

			It("sets up a proxy for the Garden server using the correct host", func() {
				Expect(fakeClient.ProxyCallCount()).To(Equal(2))
				_, proxyTo := fakeClient.ProxyArgsForCall(0)
				Expect(proxyTo).To(Equal("1.2.3.4:7777"))

				_, proxyTo = fakeClient.ProxyArgsForCall(1)
				Expect(proxyTo).To(Equal("5.6.7.8:7788"))

			})
		})
	})

	var _ = Describe("Forward", func() {

	})

	var _ = Describe("Register", func() {

	})

	var _ = Describe("Delete Worker", func() {

	})

	var _ = Describe("Retire", func() {
		var (
			signals   chan os.Signal
			ready     chan<- struct{}
			retireErr error
			exited    chan error
		)

		JustBeforeEach(func() {
			signals = make(chan os.Signal, 1)
			ready = make(chan struct{}, 1)
		})

		AfterEach(func() {
			Expect(fakeCloseable.CloseCallCount()).To(Equal(1))
		})

		Context("when the exit channel takes time to exit", func() {
			var (
				keepAliveErr    chan error
				cancelKeepAlive chan struct{}
				wait            chan bool
			)
			BeforeEach(func() {
				keepAliveErr = make(chan error, 1)
				cancelKeepAlive = make(chan struct{}, 1)
				wait = make(chan bool, 1)
				exited = make(chan error, 1)

				fakeSession.WaitStub = func() error {
					<-wait
					return nil
				}

				fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
				go func() {
					signals <- syscall.SIGKILL
					exited <- beacon.RetireWorker(signals, make(chan struct{}, 1))
				}()

			})

			It("closes the session and waits for it to shut down", func() {
				Consistently(exited).ShouldNot(Receive()) // should be blocking on exit channel
				go func() {
					wait <- false
				}()
				Eventually(exited).Should(Receive()) // should stop blocking
				Expect(fakeSession.CloseCallCount()).To(Equal(2))
			})
		})
		Context("when exiting immediately", func() {

			JustBeforeEach(func() {
				retireErr = beacon.RetireWorker(signals, ready)
			})

			Context("when waiting on the session errors", func() {
				BeforeEach(func() {
					fakeSession.WaitReturns(errors.New("fail"))
				})
				It("returns the error", func() {
					Expect(retireErr).To(Equal(errors.New("fail")))
				})
			})

			Context("when the runner recieves a signal", func() {
				var (
					keepAliveErr    chan error
					cancelKeepAlive chan struct{}
				)
				BeforeEach(func() {
					keepAliveErr = make(chan error, 1)
					cancelKeepAlive = make(chan struct{}, 1)

					wait := make(chan bool, 1)
					fakeSession.WaitStub = func() error {
						<-wait
						return nil
					}

					fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
					go func() {
						signals <- syscall.SIGKILL
						wait <- false
					}()

				})

				It("stops the keepalive", func() {
					Eventually(cancelKeepAlive).Should(BeClosed())
				})
			})

			Context("when keeping the connection alive errors", func() {
				var (
					keepAliveErr    chan error
					cancelKeepAlive chan<- struct{}
				)

				BeforeEach(func() {
					wait := make(chan bool, 1)
					fakeSession.WaitStub = func() error {
						<-wait
						return nil
					}

					keepAliveErr = make(chan error, 1)
					cancelKeepAlive = make(chan struct{}, 1)

					fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
					go func() {
						keepAliveErr <- errors.New("keepalive fail")
					}()
				})

				It("returns the error", func() {
					Expect(retireErr).To(Equal(errors.New("keepalive fail")))
				})
			})

			It("sets up a proxy for the Garden server using the correct host", func() {
				Expect(fakeClient.ProxyCallCount()).To(Equal(2))
				_, proxyTo := fakeClient.ProxyArgsForCall(0)
				Expect(proxyTo).To(Equal("1.2.3.4:7777"))

				_, proxyTo = fakeClient.ProxyArgsForCall(1)
				Expect(proxyTo).To(Equal("5.6.7.8:7788"))
			})
		})
	})

	var _ = Describe("Land", func() {
		var (
			signals chan os.Signal
			ready   chan<- struct{}
			landErr error
			exited  chan error
		)

		JustBeforeEach(func() {
			signals = make(chan os.Signal, 1)
			ready = make(chan struct{}, 1)
		})

		AfterEach(func() {
			Expect(fakeCloseable.CloseCallCount()).To(Equal(1))
		})

		Context("when the exit channel takes time to exit", func() {
			var (
				keepAliveErr    chan error
				cancelKeepAlive chan struct{}
				wait            chan bool
			)
			BeforeEach(func() {
				keepAliveErr = make(chan error, 1)
				cancelKeepAlive = make(chan struct{}, 1)
				wait = make(chan bool, 1)
				exited = make(chan error, 1)

				fakeSession.WaitStub = func() error {
					<-wait
					signals <- syscall.SIGKILL
					return nil
				}

				fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
				go func() {
					exited <- beacon.RetireWorker(signals, make(chan struct{}, 1))
				}()

			})

			It("closes the session and waits for it to shut down", func() {
				Consistently(exited).ShouldNot(Receive()) // should be blocking on exit channel
				go func() {
					wait <- false
				}()
				Eventually(exited).Should(Receive()) // should stop blocking
				Expect(fakeSession.CloseCallCount()).To(Equal(2))
			})
		})
		Context("when exiting immediately", func() {

			JustBeforeEach(func() {
				landErr = beacon.LandWorker(signals, ready)
			})

			Context("when waiting on the session errors", func() {
				BeforeEach(func() {
					fakeSession.WaitReturns(errors.New("fail"))
				})
				It("returns the error", func() {
					Expect(landErr).To(Equal(errors.New("fail")))
				})
			})

			Context("when the runner recieves a signal", func() {
				var (
					keepAliveErr    chan error
					cancelKeepAlive chan struct{}
				)
				BeforeEach(func() {
					keepAliveErr = make(chan error, 1)
					cancelKeepAlive = make(chan struct{}, 1)

					wait := make(chan bool, 1)
					fakeSession.WaitStub = func() error {
						<-wait
						return nil
					}

					fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
					go func() {
						signals <- syscall.SIGKILL
						wait <- false
					}()

				})

				It("stops the keepalive", func() {
					Eventually(cancelKeepAlive).Should(BeClosed())
				})
			})

			Context("when keeping the connection alive errors", func() {
				var (
					keepAliveErr    chan error
					cancelKeepAlive chan<- struct{}
				)

				BeforeEach(func() {
					wait := make(chan bool, 1)
					fakeSession.WaitStub = func() error {
						<-wait
						return nil
					}

					keepAliveErr = make(chan error, 1)
					cancelKeepAlive = make(chan struct{}, 1)

					fakeClient.KeepAliveReturns(keepAliveErr, cancelKeepAlive)
					go func() {
						keepAliveErr <- errors.New("keepalive fail")
					}()
				})

				It("returns the error", func() {
					Expect(landErr).To(Equal(errors.New("keepalive fail")))
				})
			})

			It("sets up a proxy for the Garden server using the correct host", func() {
				Expect(fakeClient.ProxyCallCount()).To(Equal(2))
				_, proxyTo := fakeClient.ProxyArgsForCall(0)
				Expect(proxyTo).To(Equal("1.2.3.4:7777"))

				_, proxyTo = fakeClient.ProxyArgsForCall(1)
				Expect(proxyTo).To(Equal("5.6.7.8:7788"))
			})
		})
	})

	var _ = Describe("Sweep", func() {
		var (
			err                error
			reaperServer       *ghttp.Server
			baggageclaimServer *ghttp.Server
		)

		BeforeEach(func() {
			baggageclaimServer = ghttp.NewServer()
			reaperServer = ghttp.NewServer()
			beacon.ReaperAddr = reaperServer.URL()
			beacon.BaggageclaimForwardAddr = baggageclaimServer.URL()
			reaperServer.Reset()
			baggageclaimServer.Reset()
		})

		AfterEach(func() {
			reaperServer.Close()
			baggageclaimServer.Close()
		})

		JustBeforeEach(func() {
			err = beacon.MarkandSweepContainersandVolumes()
		})

		Context("when session returns error", func() {
			BeforeEach(func() {
				reaperServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/list"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, nil),
					),
				)
				baggageclaimServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, nil),
					),
				)
				fakeSession.OutputReturns(nil, errors.New("fail"))
			})
			It("returns the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fail"))
				Expect(fakeCloseable.CloseCallCount()).To(Equal(4))
			})
		})

		Context("when bad json is returned from reaper and baggageclaim", func() {
			BeforeEach(func() {
				reaperServer.Reset()
				baggageclaimServer.Reset()

				reaperServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/list"),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)

				baggageclaimServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes"),
						ghttp.RespondWith(http.StatusOK, nil),
					),
				)

				fakeSession.OutputStub = func(cmd string) ([]byte, error) {
					if cmd == tsa.ReportContainers {
						return nil, errors.New("bad-err")
					}
					if cmd == tsa.SweepContainers {
						return []byte("bad-json"), nil
					}
					if cmd == tsa.ReportVolumes {
						return nil, errors.New("bad-err")
					}
					if cmd == tsa.SweepVolumes {
						return []byte("bad-json"), nil
					}
					time.Sleep(1 * time.Second)
					return nil, nil
				}
			})
			It("returns the error", func() {
				Expect(len(reaperServer.ReceivedRequests())).To(Equal(1))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid character"))
				Expect(fakeCloseable.CloseCallCount()).To(Equal(4))
			})
		})

		// Happy path
		Context("when reaper server and baggageclaim server are configured", func() {
			Context("when handles are returned", func() {
				var (
					buf      bytes.Buffer
					handleB2 []byte
				)
				BeforeEach(func() {
					handles := []string{"handle1", "handle2"}
					handleBytes, err := json.Marshal(handles)
					Expect(err).NotTo(HaveOccurred())
					err = json.NewEncoder(&buf).Encode(handles)
					Expect(err).NotTo(HaveOccurred())
					handleB2, err = ioutil.ReadAll(&buf)
					Expect(err).NotTo(HaveOccurred())
					fakeSession.OutputReturns(handleBytes, nil)

					reaperServer.Reset()
					reaperServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/containers/destroy"),
							ghttp.VerifyJSON("[\"handle1\",\"handle2\"]"),
							ghttp.RespondWith(http.StatusNoContent, nil),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/containers/list"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, handles),
						),
					)

					baggageclaimServer.Reset()
					baggageclaimServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes/destroy"),
							ghttp.VerifyBody(handleB2),
							ghttp.RespondWith(http.StatusNoContent, nil),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, []volume.Volume{
								{
									Handle: "handle1",
								},
								{
									Handle: "handle2",
								},
							}),
						),
					)
				})
				It("garbage collects the containers", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeSession.OutputCallCount()).To(Equal(4))
					sweepConCmd := fakeSession.OutputArgsForCall(0)
					Expect(sweepConCmd).To(Equal("sweep-containers"))
					reportConCmd := fakeSession.OutputArgsForCall(1)
					Expect(reportConCmd).To(Equal("report-containers handle1 handle2"))
					sweepVolCmd := fakeSession.OutputArgsForCall(2)
					Expect(sweepVolCmd).To(Equal("sweep-volumes"))
					reportVolCmd := fakeSession.OutputArgsForCall(3)
					Expect(reportVolCmd).To(Equal("report-volumes handle1 handle2"))
				})
			})

			Context("when reaper server and baggageclaim servers return error", func() {
				BeforeEach(func() {
					fakeSession.OutputStub = func(cmd string) ([]byte, error) {
						if strings.Contains(cmd, "sweep") {
							handles := []string{"handle1", "handle2"}
							handleBytes, err := json.Marshal(handles)
							Expect(err).NotTo(HaveOccurred())
							return handleBytes, nil
						}
						return nil, nil
					}

					reaperServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/containers/destroy"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/containers/list"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)

					baggageclaimServer.Reset()
					baggageclaimServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes/destroy"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)
				})

				It("returns the error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("received-500-response"))
					Expect(fakeCloseable.CloseCallCount()).To(Equal(4))
				})
			})

			Context("when only reaper server return error", func() {
				BeforeEach(func() {
					handles := []string{"handle1", "handle2"}
					handleBytes, err := json.Marshal(handles)
					Expect(err).NotTo(HaveOccurred())
					fakeSession.OutputReturns(handleBytes, nil)

					reaperServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/containers/destroy"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/containers/list"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)

					baggageclaimServer.Reset()
					vol1 := volume.Volume{
						Handle: "handle1",
					}
					volBytes, err := json.Marshal(volume.Volumes{vol1})

					jsonHeader := map[string][]string{"Content-Type": []string{"application/json"}}
					baggageclaimServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes/destroy"),
							ghttp.RespondWith(http.StatusNoContent, nil),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes"),
							ghttp.RespondWith(http.StatusOK, volBytes, jsonHeader),
						),
					)
				})

				It("calls baggageclaim server for further calls on sweep and report", func() {
					Expect(err).To(HaveOccurred())

					Expect(err.Error()).To(ContainSubstring("sweep-cnt-err"))
					Expect(err.Error()).To(ContainSubstring("report-cnt-err"))

					Expect(err.Error()).ToNot(ContainSubstring("sweep-vol-err"))
					Expect(err.Error()).ToNot(ContainSubstring("report-vol-err"))

					Expect(len(baggageclaimServer.ReceivedRequests())).To(Equal(2))
				})
			})

			Context("when reaper server is not running", func() {
				BeforeEach(func() {
					handles := []string{"handle1", "handle2"}
					handleBytes, err := json.Marshal(handles)
					Expect(err).NotTo(HaveOccurred())
					fakeSession.OutputReturns(handleBytes, nil)
					reaperServer.Close()
					baggageclaimServer.Close()
				})

				It("returns the error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("connection refused"))
					Expect(fakeCloseable.CloseCallCount()).To(Equal(4))
				})
			})
		})
	})
})

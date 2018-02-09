package beacon_test

import (
	"errors"
	"fmt"
	"os"

	. "github.com/concourse/worker/beacon"
	"github.com/concourse/worker/beacon/beaconfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Beacon", func() {

	var (
		beacon      Beacon
		fakeClient  *beaconfakes.FakeClient
		fakeSession *beaconfakes.FakeSession
	)

	BeforeEach(func() {
		fakeClient = new(beaconfakes.FakeClient)
		fakeSession = new(beaconfakes.FakeSession)
		fakeClient.NewSessionReturns(fakeSession, nil)

		beacon = Beacon{
			Client: fakeClient,
		}
	})

	AfterEach(func() {
		Expect(fakeClient.CloseCallCount()).To(Equal(1))
	})

	var _ = Describe("Register", func() {
		var (
			signals     <-chan os.Signal
			ready       chan<- struct{}
			registerErr error
		)

		JustBeforeEach(func() {
			signals = make(chan os.Signal, 1)
			ready = make(chan struct{}, 1)
			fmt.Fprintln(GinkgoWriter, beacon.RegistrationMode)
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

		Context("when the registration mode is 'forward'", func() {
			BeforeEach(func() {
				beacon = Beacon{
					Client:           fakeClient,
					RegistrationMode: Forward,
				}

			})

			It("Forwards the worker's Garden and Baggageclaim to TSA", func() {
				By("using the forward-worker command")
				Expect(fakeSession.StartCallCount()).To(Equal(1))
				Expect(fakeSession.StartArgsForCall(0)).To(Equal("forward-worker --garden 0.0.0.0:7777 --baggageclaim 0.0.0.0:7788"))
			})
		})

		Context("when the registration mode is 'direct'", func() {
			BeforeEach(func() {
				beacon = Beacon{
					Client:           fakeClient,
					RegistrationMode: Direct,
				}
			})

			It("Registers directly with the TSA", func() {
				By("using the register-worker command")
				Expect(fakeSession.StartCallCount()).To(Equal(1))
				Expect(fakeSession.StartArgsForCall(0)).To(Equal("register-worker"))
			})
		})
	})

	var _ = Describe("Forward", func() {

	})

	var _ = Describe("Register", func() {

	})

	var _ = Describe("Retire", func() {

	})

	var _ = Describe("Land", func() {

	})
})

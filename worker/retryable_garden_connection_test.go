package worker_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	fconn "github.com/cloudfoundry-incubator/garden/client/connection/fakes"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Retryable", func() {
	var (
		innerConnection *fconn.FakeConnection
		retryPolicy     *fakes.FakeRetryPolicy
		sleeper         *fakes.FakeSleeper

		conn gconn.Connection
	)

	BeforeEach(func() {
		innerConnection = new(fconn.FakeConnection)
		retryPolicy = new(fakes.FakeRetryPolicy)
		sleeper = new(fakes.FakeSleeper)

		conn = worker.RetryableConnection{
			Connection:  innerConnection,
			Logger:      lagertest.NewTestLogger("retryable-connection"),
			Sleeper:     sleeper,
			RetryPolicy: retryPolicy,
		}
	})

	retryableErrors := []error{
		syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.ETIMEDOUT,
		errors.New("no such host"),
		errors.New("remote error: handshake failure"),
		// io.ErrUnexpectedEOF,
		// io.EOF,
	}

	itRetries := func(action func() error, resultIn func(error), attempts func() int, example func()) {
		var errResult error

		JustBeforeEach(func() {
			errResult = action()
		})

		for _, err := range retryableErrors {
			retryableError := err

			Context("when the error is "+retryableError.Error(), func() {
				BeforeEach(func() {
					resultIn(retryableError)
				})

				Context("as long as the backoff policy returns true", func() {
					BeforeEach(func() {
						durations := make(chan time.Duration, 3)
						durations <- time.Second
						durations <- 2 * time.Second
						durations <- 1000 * time.Second
						close(durations)

						retryPolicy.DelayForStub = func(failedAttempts uint) (time.Duration, bool) {
							Expect(attempts()).To(Equal(int(failedAttempts)))

							select {
							case d, ok := <-durations:
								return d, ok
							}
						}
					})

					It("continuously retries with an increasing attempt count", func() {
						Expect(retryPolicy.DelayForCallCount()).To(Equal(4))
						Expect(sleeper.SleepCallCount()).To(Equal(3))

						Expect(retryPolicy.DelayForArgsForCall(0)).To(Equal(uint(1)))
						Expect(sleeper.SleepArgsForCall(0)).To(Equal(time.Second))

						Expect(retryPolicy.DelayForArgsForCall(1)).To(Equal(uint(2)))
						Expect(sleeper.SleepArgsForCall(1)).To(Equal(2 * time.Second))

						Expect(retryPolicy.DelayForArgsForCall(2)).To(Equal(uint(3)))
						Expect(sleeper.SleepArgsForCall(2)).To(Equal(1000 * time.Second))

						Expect(errResult).To(Equal(retryableError))
					})
				})
			})
		}

		Context("when the error is not retryable", func() {
			var returnedErr error

			BeforeEach(func() {
				retryPolicy.DelayForReturns(0, true)

				returnedErr = errors.New("oh no!")
				resultIn(returnedErr)
			})

			It("propagates the error", func() {
				Expect(errResult).To(Equal(returnedErr))
			})

			It("does not retry", func() {
				Expect(attempts()).To(Equal(1))
			})
		})

		Context("when there is no error", func() {
			BeforeEach(func() {
				resultIn(nil)
			})

			example()

			It("does not error", func() {
				Expect(errResult).NotTo(HaveOccurred())
			})
		})
	}

	Describe("Capacity", func() {
		capacity := garden.Capacity{
			MemoryInBytes: 34567809,
			DiskInBytes:   7834506,
		}

		var gotCapacity garden.Capacity

		itRetries(func() error {
			var err error
			gotCapacity, err = conn.Capacity()
			return err
		}, func(err error) {
			innerConnection.CapacityReturns(capacity, err)
		}, func() int {
			return innerConnection.CapacityCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.CapacityCallCount()).To(Equal(1))
			})

			It("returns the capacity", func() {
				Expect(gotCapacity).To(Equal(capacity))
			})
		})
	})

	Describe("List", func() {
		properties := garden.Properties{
			"A": "B",
		}

		var gotHandles []string

		itRetries(func() error {
			var err error
			gotHandles, err = conn.List(properties)
			return err
		}, func(err error) {
			innerConnection.ListReturns([]string{"a", "b"}, err)
		}, func() int {
			return innerConnection.ListCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.ListCallCount()).To(Equal(1))

				listedProperties := innerConnection.ListArgsForCall(0)
				Expect(listedProperties).To(Equal(properties))
			})

			It("returns the handles", func() {
				Expect(gotHandles).To(Equal([]string{"a", "b"}))
			})
		})
	})

	Describe("Info", func() {
		handle := "doorknob"

		var gotInfo garden.ContainerInfo

		itRetries(func() error {
			var err error
			gotInfo, err = conn.Info(handle)
			return err
		}, func(err error) {
			innerConnection.InfoReturns(garden.ContainerInfo{
				State: "chillin",
			}, err)
		}, func() int {
			return innerConnection.InfoCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.InfoCallCount()).To(Equal(1))

				infoedHandle := innerConnection.InfoArgsForCall(0)
				Expect(infoedHandle).To(Equal(handle))
			})

			It("returns the info", func() {
				Expect(gotInfo).To(Equal(garden.ContainerInfo{
					State: "chillin",
				}))

			})
		})
	})

	Describe("NetIn", func() {
		var hostPort uint32 = 23456
		var containerPort uint32 = 34567

		var gotHostPort uint32
		var gotContainerPort uint32

		itRetries(func() error {
			var err error
			gotHostPort, gotContainerPort, err = conn.NetIn("la-contineur", 1, 2)
			return err
		}, func(err error) {
			innerConnection.NetInReturns(hostPort, containerPort, err)
		}, func() int {
			return innerConnection.NetInCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.NetInCallCount()).To(Equal(1))

				handle, hostPort, containerPort := innerConnection.NetInArgsForCall(0)
				Expect(handle).To(Equal("la-contineur"))
				Expect(hostPort).To(Equal(uint32(1)))
				Expect(containerPort).To(Equal(uint32(2)))
			})

			It("returns the ports", func() {
				Expect(gotHostPort).To(Equal(hostPort))
				Expect(gotContainerPort).To(Equal(containerPort))
			})
		})
	})

	Describe("NetOut", func() {
		netOutRule := garden.NetOutRule{
			Protocol: garden.ProtocolTCP,
			Ports: []garden.PortRange{
				garden.PortRangeFromPort(13253),
			},
		}

		itRetries(func() error {
			return conn.NetOut("la-contineur", netOutRule)
		}, func(err error) {
			innerConnection.NetOutReturns(err)
		}, func() int {
			return innerConnection.NetOutCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.NetOutCallCount()).To(Equal(1))

				handle, calledNetOutRule := innerConnection.NetOutArgsForCall(0)
				Expect(handle).To(Equal("la-contineur"))
				Expect(calledNetOutRule).To(Equal(netOutRule))
			})
		})
	})

	Describe("CurrentBandwidthLimits", func() {
		handle := "suitcase"

		limits := garden.BandwidthLimits{
			RateInBytesPerSecond: 234,
		}

		var gotLimits garden.BandwidthLimits

		itRetries(func() error {
			var err error
			gotLimits, err = conn.CurrentBandwidthLimits(handle)
			return err
		}, func(err error) {
			innerConnection.CurrentBandwidthLimitsReturns(limits, err)
		}, func() int {
			return innerConnection.CurrentBandwidthLimitsCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.CurrentBandwidthLimitsCallCount()).To(Equal(1))

				calledHandle := innerConnection.CurrentBandwidthLimitsArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
			})

			It("returns the limits", func() {
				Expect(gotLimits).To(Equal(limits))
			})
		})
	})

	Describe("CurrentCPULimits", func() {
		handle := "suitcase"

		limits := garden.CPULimits{
			LimitInShares: 7,
		}

		var gotLimits garden.CPULimits

		itRetries(func() error {
			var err error
			gotLimits, err = conn.CurrentCPULimits(handle)
			return err
		}, func(err error) {
			innerConnection.CurrentCPULimitsReturns(limits, err)
		}, func() int {
			return innerConnection.CurrentCPULimitsCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.CurrentCPULimitsCallCount()).To(Equal(1))

				calledHandle := innerConnection.CurrentCPULimitsArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
			})

			It("returns the limits", func() {
				Expect(gotLimits).To(Equal(limits))
			})
		})
	})

	Describe("CurrentDiskLimits", func() {
		handle := "suitcase"

		limits := garden.DiskLimits{
			ByteHard: 234,
		}

		var gotLimits garden.DiskLimits

		itRetries(func() error {
			var err error
			gotLimits, err = conn.CurrentDiskLimits(handle)
			return err
		}, func(err error) {
			innerConnection.CurrentDiskLimitsReturns(limits, err)
		}, func() int {
			return innerConnection.CurrentDiskLimitsCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.CurrentDiskLimitsCallCount()).To(Equal(1))

				calledHandle := innerConnection.CurrentDiskLimitsArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
			})

			It("returns the limits", func() {
				Expect(gotLimits).To(Equal(limits))
			})
		})
	})

	Describe("CurrentMemoryLimits", func() {
		handle := "suitcase"

		limits := garden.MemoryLimits{
			LimitInBytes: 234,
		}

		var gotLimits garden.MemoryLimits

		itRetries(func() error {
			var err error
			gotLimits, err = conn.CurrentMemoryLimits(handle)
			return err
		}, func(err error) {
			innerConnection.CurrentMemoryLimitsReturns(limits, err)
		}, func() int {
			return innerConnection.CurrentMemoryLimitsCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.CurrentMemoryLimitsCallCount()).To(Equal(1))

				calledHandle := innerConnection.CurrentMemoryLimitsArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
			})

			It("returns the limits", func() {
				Expect(gotLimits).To(Equal(limits))
			})
		})
	})

	Describe("LimitBandwidth", func() {
		handle := "suitcase"

		limits := garden.BandwidthLimits{
			RateInBytesPerSecond: 234,
		}

		var gotLimits garden.BandwidthLimits

		itRetries(func() error {
			var err error
			gotLimits, err = conn.LimitBandwidth(handle, limits)
			return err
		}, func(err error) {
			innerConnection.LimitBandwidthReturns(limits, err)
		}, func() int {
			return innerConnection.LimitBandwidthCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.LimitBandwidthCallCount()).To(Equal(1))

				calledHandle, calledLimits := innerConnection.LimitBandwidthArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
				Expect(calledLimits).To(Equal(limits))
			})

			It("returns the limits", func() {
				Expect(gotLimits).To(Equal(limits))
			})
		})
	})

	Describe("LimitCPU", func() {
		handle := "suitcase"

		limits := garden.CPULimits{
			LimitInShares: 7,
		}

		var gotLimits garden.CPULimits

		itRetries(func() error {
			var err error
			gotLimits, err = conn.LimitCPU(handle, limits)
			return err
		}, func(err error) {
			innerConnection.LimitCPUReturns(limits, err)
		}, func() int {
			return innerConnection.LimitCPUCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.LimitCPUCallCount()).To(Equal(1))

				calledHandle, calledLimits := innerConnection.LimitCPUArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
				Expect(calledLimits).To(Equal(limits))
			})

			It("returns the limits", func() {
				Expect(gotLimits).To(Equal(limits))
			})
		})
	})

	Describe("LimitDisk", func() {
		handle := "suitcase"

		limits := garden.DiskLimits{
			ByteHard: 234,
		}

		var gotLimits garden.DiskLimits

		itRetries(func() error {
			var err error
			gotLimits, err = conn.LimitDisk(handle, limits)
			return err
		}, func(err error) {
			innerConnection.LimitDiskReturns(limits, err)
		}, func() int {
			return innerConnection.LimitDiskCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.LimitDiskCallCount()).To(Equal(1))

				calledHandle, calledLimits := innerConnection.LimitDiskArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
				Expect(calledLimits).To(Equal(limits))
			})

			It("returns the limits", func() {
				Expect(gotLimits).To(Equal(limits))
			})
		})
	})

	Describe("LimitMemory", func() {
		handle := "suitcase"

		limits := garden.MemoryLimits{
			LimitInBytes: 234,
		}

		var gotLimits garden.MemoryLimits

		itRetries(func() error {
			var err error
			gotLimits, err = conn.LimitMemory(handle, limits)
			return err
		}, func(err error) {
			innerConnection.LimitMemoryReturns(limits, err)
		}, func() int {
			return innerConnection.LimitMemoryCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.LimitMemoryCallCount()).To(Equal(1))

				calledHandle, calledLimits := innerConnection.LimitMemoryArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
				Expect(calledLimits).To(Equal(limits))
			})

			It("returns the limits", func() {
				Expect(gotLimits).To(Equal(limits))
			})
		})
	})

	Describe("Create", func() {
		spec := garden.ContainerSpec{
			RootFSPath: "/dev/mouse",
		}

		var gotHandle string

		itRetries(func() error {
			var err error
			gotHandle, err = conn.Create(spec)
			return err
		}, func(err error) {
			innerConnection.CreateReturns("bach", err)
		}, func() int {
			return innerConnection.CreateCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.CreateCallCount()).To(Equal(1))

				calledSpec := innerConnection.CreateArgsForCall(0)
				Expect(calledSpec).To(Equal(spec))
			})

			It("returns the container handle", func() {
				Expect(gotHandle).To(Equal("bach"))
			})
		})
	})

	Describe("Destroy", func() {
		handle := "mozart"

		itRetries(func() error {
			return conn.Destroy(handle)
		}, func(err error) {
			innerConnection.DestroyReturns(err)
		}, func() int {
			return innerConnection.DestroyCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.DestroyCallCount()).To(Equal(1))

				calledHandle := innerConnection.DestroyArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
			})
		})
	})

	Describe("Property", func() {
		handle := "suitcase"
		property := "dfghjkl"

		var gotValue string

		itRetries(func() error {
			var err error
			gotValue, err = conn.Property(handle, property)
			return err
		}, func(err error) {
			innerConnection.PropertyReturns("some-value", err)
		}, func() int {
			return innerConnection.PropertyCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.PropertyCallCount()).To(Equal(1))

				calledHandle, calledProperty := innerConnection.PropertyArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
				Expect(calledProperty).To(Equal(property))
			})

			It("returns the limits", func() {
				Expect(gotValue).To(Equal("some-value"))
			})
		})
	})

	Describe("SetProperty", func() {
		itRetries(func() error {
			return conn.SetProperty("la-contineur", "some-name", "some-value")
		}, func(err error) {
			innerConnection.SetPropertyReturns(err)
		}, func() int {
			return innerConnection.SetPropertyCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.SetPropertyCallCount()).To(Equal(1))

				handle, setName, setValue := innerConnection.SetPropertyArgsForCall(0)
				Expect(handle).To(Equal("la-contineur"))
				Expect(setName).To(Equal("some-name"))
				Expect(setValue).To(Equal("some-value"))
			})
		})
	})

	Describe("RemoveProperty", func() {
		itRetries(func() error {
			return conn.RemoveProperty("la-contineur", "some-name")
		}, func(err error) {
			innerConnection.RemovePropertyReturns(err)
		}, func() int {
			return innerConnection.RemovePropertyCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.RemovePropertyCallCount()).To(Equal(1))

				handle, setName := innerConnection.RemovePropertyArgsForCall(0)
				Expect(handle).To(Equal("la-contineur"))
				Expect(setName).To(Equal("some-name"))
			})
		})
	})

	Describe("Stop", func() {
		itRetries(func() error {
			return conn.Stop("la-contineur", true)
		}, func(err error) {
			innerConnection.StopReturns(err)
		}, func() int {
			return innerConnection.StopCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.StopCallCount()).To(Equal(1))

				handle, kill := innerConnection.StopArgsForCall(0)
				Expect(handle).To(Equal("la-contineur"))
				Expect(kill).To(Equal(true))
			})
		})
	})

	Describe("StreamIn", func() {
		BeforeEach(func() {
			durations := make(chan time.Duration, 1)
			durations <- time.Second
			close(durations)

			retryPolicy.DelayForStub = func(failedAttempts uint) (time.Duration, bool) {
				select {
				case d, ok := <-durations:
					return d, ok
				}
			}
		})

		It("calls through to the inner connection", func() {
			reader := &bytes.Buffer{}

			err := conn.StreamIn("beethoven", garden.StreamInSpec{
				Path:      "/dev/sound",
				User:      "bach",
				TarStream: reader,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(innerConnection.StreamInCallCount()).To(Equal(1))
			handle, spec := innerConnection.StreamInArgsForCall(0)
			Expect(handle).To(Equal("beethoven"))
			Expect(spec.Path).To(Equal("/dev/sound"))
			Expect(spec.User).To(Equal("bach"))
			Expect(spec.TarStream).To(Equal(reader))
		})

		It("does not retry as the other end of the connection may have already started reading the body", func() {
			reader := &bytes.Buffer{}

			innerConnection.StreamInReturns(retryableErrors[0])

			err := conn.StreamIn("beethoven", garden.StreamInSpec{
				Path:      "/dev/sound",
				User:      "bach",
				TarStream: reader,
			})
			Expect(err).To(MatchError(retryableErrors[0]))

			Expect(innerConnection.StreamInCallCount()).To(Equal(1))
		})
	})

	Describe("StreamOut", func() {
		handle := "suitcase"

		var gotReader io.ReadCloser

		itRetries(func() error {
			var err error
			gotReader, err = conn.StreamOut(handle, garden.StreamOutSpec{
				Path: "/etc/passwd",
				User: "admin",
			})
			return err
		}, func(err error) {
			innerConnection.StreamOutReturns(gbytes.NewBuffer(), err)
		}, func() int {
			return innerConnection.StreamOutCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.StreamOutCallCount()).To(Equal(1))

				calledHandle, calledSpec := innerConnection.StreamOutArgsForCall(0)
				Expect(calledHandle).To(Equal(handle))
				Expect(calledSpec.Path).To(Equal("/etc/passwd"))
				Expect(calledSpec.User).To(Equal("admin"))
			})

			It("returns the reader", func() {
				Expect(gotReader).To(Equal(gbytes.NewBuffer()))
			})
		})
	})

	Describe("Attach", func() {
		var (
			fakeProcess *gfakes.FakeProcess
			process     garden.Process
		)

		processIO := garden.ProcessIO{
			Stdout: gbytes.NewBuffer(),
		}

		BeforeEach(func() {
			fakeProcess = new(gfakes.FakeProcess)
			fakeProcess.IDReturns(6)
		})

		itRetries(func() error {
			var err error
			process, err = conn.Attach("la-contineur", 6, processIO)
			return err
		}, func(err error) {
			innerConnection.AttachReturns(fakeProcess, err)
		}, func() int {
			return innerConnection.AttachCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.AttachCallCount()).To(Equal(1))

				handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
				Expect(handle).To(Equal("la-contineur"))
				Expect(processID).To(Equal(uint32(6)))
				Expect(calledProcessIO).To(Equal(processIO))
			})

			Describe("the process", func() {
				Describe("Wait", func() {
					BeforeEach(func() {
						errs := make(chan error, 1)
						errs <- fmt.Errorf("connection: decode failed: %s", io.EOF)
						close(errs)

						fakeProcess.WaitStub = func() (int, error) {
							err := <-errs
							if err == nil {
								return 42, nil
							}

							return 0, err
						}
					})

					It("reattaches on EOF", func() {
						result, err := process.Wait()
						Expect(err).NotTo(HaveOccurred())
						Expect(result).To(Equal(42))

						Expect(innerConnection.AttachCallCount()).To(Equal(2))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(1)
						Expect(handle).To(Equal("la-contineur"))
						Expect(processID).To(Equal(uint32(6)))
						Expect(calledProcessIO).To(Equal(processIO))
					})
				})

				Describe("Signal", func() {
					BeforeEach(func() {
						errs := make(chan error, 1)
						errs <- io.EOF
						close(errs)

						fakeProcess.SignalStub = func(garden.Signal) error {
							return <-errs
						}
					})

					It("reattaches on use of closed connection", func() {
						Expect(process.Signal(garden.SignalTerminate)).To(Succeed())
						Expect(fakeProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))

						Expect(innerConnection.AttachCallCount()).To(Equal(2))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(1)
						Expect(handle).To(Equal("la-contineur"))
						Expect(processID).To(Equal(uint32(6)))
						Expect(calledProcessIO).To(Equal(processIO))
					})
				})

				Describe("SetTTY", func() {
					BeforeEach(func() {
						errs := make(chan error, 1)
						errs <- io.EOF
						close(errs)

						fakeProcess.SetTTYStub = func(garden.TTYSpec) error {
							return <-errs
						}
					})

					It("reattaches on use of closed connection", func() {
						ttySpec := garden.TTYSpec{
							WindowSize: &garden.WindowSize{Columns: 345678, Rows: 45689},
						}

						Expect(process.SetTTY(ttySpec)).To(Succeed())
						Expect(fakeProcess.SetTTYArgsForCall(0)).To(Equal(ttySpec))

						Expect(innerConnection.AttachCallCount()).To(Equal(2))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(1)
						Expect(handle).To(Equal("la-contineur"))
						Expect(processID).To(Equal(uint32(6)))
						Expect(calledProcessIO).To(Equal(processIO))
					})
				})
			})
		})
	})

	Describe("Run", func() {
		var (
			fakeProcess *gfakes.FakeProcess
			process     garden.Process
		)

		processSpec := garden.ProcessSpec{
			Path: "reboot",
		}

		processIO := garden.ProcessIO{
			Stdout: gbytes.NewBuffer(),
		}

		BeforeEach(func() {
			fakeProcess = new(gfakes.FakeProcess)
			fakeProcess.IDReturns(6)
		})

		itRetries(func() error {
			var err error
			process, err = conn.Run("la-contineur", processSpec, processIO)
			return err
		}, func(err error) {
			innerConnection.RunReturns(fakeProcess, err)
		}, func() int {
			return innerConnection.RunCallCount()
		}, func() {
			It("calls through to garden", func() {
				Expect(innerConnection.RunCallCount()).To(Equal(1))

				handle, calledProcessSpec, calledProcessIO := innerConnection.RunArgsForCall(0)
				Expect(handle).To(Equal("la-contineur"))
				Expect(calledProcessSpec).To(Equal(processSpec))
				Expect(calledProcessIO).To(Equal(processIO))
			})

			Describe("the process", func() {
				BeforeEach(func() {
					innerConnection.AttachReturns(fakeProcess, nil)
				})

				Describe("Wait", func() {
					BeforeEach(func() {
						errs := make(chan error, 1)
						errs <- io.EOF
						close(errs)

						fakeProcess.WaitStub = func() (int, error) {
							err := <-errs
							if err == nil {
								return 42, nil
							}

							return 0, err
						}
					})

					It("reattaches on EOF", func() {
						Expect(process.Wait()).To(Equal(42))

						Expect(innerConnection.AttachCallCount()).To(Equal(1))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
						Expect(handle).To(Equal("la-contineur"))
						Expect(processID).To(Equal(uint32(6)))
						Expect(calledProcessIO).To(Equal(processIO))
					})
				})

				Describe("Signal", func() {
					BeforeEach(func() {
						errs := make(chan error, 1)
						errs <- io.EOF
						close(errs)

						fakeProcess.SignalStub = func(garden.Signal) error {
							return <-errs
						}
					})

					It("reattaches on use of closed connection", func() {
						Expect(process.Signal(garden.SignalTerminate)).To(Succeed())
						Expect(fakeProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))

						Expect(innerConnection.AttachCallCount()).To(Equal(1))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
						Expect(handle).To(Equal("la-contineur"))
						Expect(processID).To(Equal(uint32(6)))
						Expect(calledProcessIO).To(Equal(processIO))
					})
				})

				Describe("SetTTY", func() {
					BeforeEach(func() {
						errs := make(chan error, 1)
						errs <- io.EOF
						close(errs)

						fakeProcess.SetTTYStub = func(garden.TTYSpec) error {
							return <-errs
						}
					})

					It("reattaches on use of closed connection", func() {
						ttySpec := garden.TTYSpec{
							WindowSize: &garden.WindowSize{Columns: 345678, Rows: 45689},
						}

						Expect(process.SetTTY(ttySpec)).To(Succeed())
						Expect(fakeProcess.SetTTYArgsForCall(0)).To(Equal(ttySpec))

						Expect(innerConnection.AttachCallCount()).To(Equal(1))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
						Expect(handle).To(Equal("la-contineur"))
						Expect(processID).To(Equal(uint32(6)))
						Expect(calledProcessIO).To(Equal(processIO))
					})
				})
			})
		})
	})
})

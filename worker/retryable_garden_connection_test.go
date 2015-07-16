package worker_test

import (
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
	"github.com/pivotal-golang/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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
			Logger:      lager.NewLogger("dumb"),
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
							Ω(attempts()).Should(Equal(int(failedAttempts)))

							select {
							case d, ok := <-durations:
								return d, ok
							}
						}
					})

					It("continuously retries with an increasing attempt count", func() {
						Ω(retryPolicy.DelayForCallCount()).Should(Equal(4))
						Ω(sleeper.SleepCallCount()).Should(Equal(3))

						Ω(retryPolicy.DelayForArgsForCall(0)).Should(Equal(uint(1)))
						Ω(sleeper.SleepArgsForCall(0)).Should(Equal(time.Second))

						Ω(retryPolicy.DelayForArgsForCall(1)).Should(Equal(uint(2)))
						Ω(sleeper.SleepArgsForCall(1)).Should(Equal(2 * time.Second))

						Ω(retryPolicy.DelayForArgsForCall(2)).Should(Equal(uint(3)))
						Ω(sleeper.SleepArgsForCall(2)).Should(Equal(1000 * time.Second))

						Ω(errResult).Should(Equal(retryableError))
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
				Ω(errResult).Should(Equal(returnedErr))
			})

			It("does not retry", func() {
				Ω(attempts()).Should(Equal(1))
			})
		})

		Context("when there is no error", func() {
			BeforeEach(func() {
				resultIn(nil)
			})

			example()

			It("does not error", func() {
				Ω(errResult).ShouldNot(HaveOccurred())
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
				Ω(innerConnection.CapacityCallCount()).Should(Equal(1))
			})

			It("returns the capacity", func() {
				Ω(gotCapacity).Should(Equal(capacity))
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
				Ω(innerConnection.ListCallCount()).Should(Equal(1))

				listedProperties := innerConnection.ListArgsForCall(0)
				Ω(listedProperties).Should(Equal(properties))
			})

			It("returns the handles", func() {
				Ω(gotHandles).Should(Equal([]string{"a", "b"}))
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
				Ω(innerConnection.InfoCallCount()).Should(Equal(1))

				infoedHandle := innerConnection.InfoArgsForCall(0)
				Ω(infoedHandle).Should(Equal(handle))
			})

			It("returns the info", func() {
				Ω(gotInfo).Should(Equal(garden.ContainerInfo{
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
				Ω(innerConnection.NetInCallCount()).Should(Equal(1))

				handle, hostPort, containerPort := innerConnection.NetInArgsForCall(0)
				Ω(handle).Should(Equal("la-contineur"))
				Ω(hostPort).Should(Equal(uint32(1)))
				Ω(containerPort).Should(Equal(uint32(2)))
			})

			It("returns the ports", func() {
				Ω(gotHostPort).Should(Equal(hostPort))
				Ω(gotContainerPort).Should(Equal(containerPort))
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
				Ω(innerConnection.NetOutCallCount()).Should(Equal(1))

				handle, calledNetOutRule := innerConnection.NetOutArgsForCall(0)
				Ω(handle).Should(Equal("la-contineur"))
				Ω(calledNetOutRule).Should(Equal(netOutRule))
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
				Ω(innerConnection.CurrentBandwidthLimitsCallCount()).Should(Equal(1))

				calledHandle := innerConnection.CurrentBandwidthLimitsArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
			})

			It("returns the limits", func() {
				Ω(gotLimits).Should(Equal(limits))
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
				Ω(innerConnection.CurrentCPULimitsCallCount()).Should(Equal(1))

				calledHandle := innerConnection.CurrentCPULimitsArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
			})

			It("returns the limits", func() {
				Ω(gotLimits).Should(Equal(limits))
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
				Ω(innerConnection.CurrentDiskLimitsCallCount()).Should(Equal(1))

				calledHandle := innerConnection.CurrentDiskLimitsArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
			})

			It("returns the limits", func() {
				Ω(gotLimits).Should(Equal(limits))
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
				Ω(innerConnection.CurrentMemoryLimitsCallCount()).Should(Equal(1))

				calledHandle := innerConnection.CurrentMemoryLimitsArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
			})

			It("returns the limits", func() {
				Ω(gotLimits).Should(Equal(limits))
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
				Ω(innerConnection.LimitBandwidthCallCount()).Should(Equal(1))

				calledHandle, calledLimits := innerConnection.LimitBandwidthArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
				Ω(calledLimits).Should(Equal(limits))
			})

			It("returns the limits", func() {
				Ω(gotLimits).Should(Equal(limits))
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
				Ω(innerConnection.LimitCPUCallCount()).Should(Equal(1))

				calledHandle, calledLimits := innerConnection.LimitCPUArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
				Ω(calledLimits).Should(Equal(limits))
			})

			It("returns the limits", func() {
				Ω(gotLimits).Should(Equal(limits))
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
				Ω(innerConnection.LimitDiskCallCount()).Should(Equal(1))

				calledHandle, calledLimits := innerConnection.LimitDiskArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
				Ω(calledLimits).Should(Equal(limits))
			})

			It("returns the limits", func() {
				Ω(gotLimits).Should(Equal(limits))
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
				Ω(innerConnection.LimitMemoryCallCount()).Should(Equal(1))

				calledHandle, calledLimits := innerConnection.LimitMemoryArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
				Ω(calledLimits).Should(Equal(limits))
			})

			It("returns the limits", func() {
				Ω(gotLimits).Should(Equal(limits))
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
				Ω(innerConnection.CreateCallCount()).Should(Equal(1))

				calledSpec := innerConnection.CreateArgsForCall(0)
				Ω(calledSpec).Should(Equal(spec))
			})

			It("returns the container handle", func() {
				Ω(gotHandle).Should(Equal("bach"))
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
				Ω(innerConnection.DestroyCallCount()).Should(Equal(1))

				calledHandle := innerConnection.DestroyArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
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
				Ω(innerConnection.PropertyCallCount()).Should(Equal(1))

				calledHandle, calledProperty := innerConnection.PropertyArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
				Ω(calledProperty).Should(Equal(property))
			})

			It("returns the limits", func() {
				Ω(gotValue).Should(Equal("some-value"))
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
				Ω(innerConnection.SetPropertyCallCount()).Should(Equal(1))

				handle, setName, setValue := innerConnection.SetPropertyArgsForCall(0)
				Ω(handle).Should(Equal("la-contineur"))
				Ω(setName).Should(Equal("some-name"))
				Ω(setValue).Should(Equal("some-value"))
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
				Ω(innerConnection.RemovePropertyCallCount()).Should(Equal(1))

				handle, setName := innerConnection.RemovePropertyArgsForCall(0)
				Ω(handle).Should(Equal("la-contineur"))
				Ω(setName).Should(Equal("some-name"))
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
				Ω(innerConnection.StopCallCount()).Should(Equal(1))

				handle, kill := innerConnection.StopArgsForCall(0)
				Ω(handle).Should(Equal("la-contineur"))
				Ω(kill).Should(Equal(true))
			})
		})
	})

	Describe("StreamIn", func() {
		reader := gbytes.NewBuffer()

		itRetries(func() error {
			return conn.StreamIn("beethoven", garden.StreamInSpec{
				Path:      "/dev/sound",
				User:      "bach",
				TarStream: reader,
			})
		}, func(err error) {
			innerConnection.StreamInReturns(err)
		}, func() int {
			return innerConnection.StreamInCallCount()
		}, func() {
			It("calls through to garden", func() {
				Ω(innerConnection.StreamInCallCount()).Should(Equal(1))

				handle, spec := innerConnection.StreamInArgsForCall(0)
				Ω(handle).Should(Equal("beethoven"))
				Ω(spec.Path).Should(Equal("/dev/sound"))
				Ω(spec.User).Should(Equal("bach"))
				Ω(spec.TarStream).Should(Equal(reader))
			})
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
				Ω(innerConnection.StreamOutCallCount()).Should(Equal(1))

				calledHandle, calledSpec := innerConnection.StreamOutArgsForCall(0)
				Ω(calledHandle).Should(Equal(handle))
				Ω(calledSpec.Path).Should(Equal("/etc/passwd"))
				Ω(calledSpec.User).Should(Equal("admin"))
			})

			It("returns the reader", func() {
				Ω(gotReader).Should(Equal(gbytes.NewBuffer()))
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
				Ω(innerConnection.AttachCallCount()).Should(Equal(1))

				handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
				Ω(handle).Should(Equal("la-contineur"))
				Ω(processID).Should(Equal(uint32(6)))
				Ω(calledProcessIO).Should(Equal(processIO))
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
						Ω(err).ShouldNot(HaveOccurred())
						Ω(result).Should(Equal(42))

						Ω(innerConnection.AttachCallCount()).Should(Equal(2))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(1)
						Ω(handle).Should(Equal("la-contineur"))
						Ω(processID).Should(Equal(uint32(6)))
						Ω(calledProcessIO).Should(Equal(processIO))
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
						Ω(process.Signal(garden.SignalTerminate)).Should(Succeed())
						Ω(fakeProcess.SignalArgsForCall(0)).Should(Equal(garden.SignalTerminate))

						Ω(innerConnection.AttachCallCount()).Should(Equal(2))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(1)
						Ω(handle).Should(Equal("la-contineur"))
						Ω(processID).Should(Equal(uint32(6)))
						Ω(calledProcessIO).Should(Equal(processIO))
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

						Ω(process.SetTTY(ttySpec)).Should(Succeed())
						Ω(fakeProcess.SetTTYArgsForCall(0)).Should(Equal(ttySpec))

						Ω(innerConnection.AttachCallCount()).Should(Equal(2))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(1)
						Ω(handle).Should(Equal("la-contineur"))
						Ω(processID).Should(Equal(uint32(6)))
						Ω(calledProcessIO).Should(Equal(processIO))
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
				Ω(innerConnection.RunCallCount()).Should(Equal(1))

				handle, calledProcessSpec, calledProcessIO := innerConnection.RunArgsForCall(0)
				Ω(handle).Should(Equal("la-contineur"))
				Ω(calledProcessSpec).Should(Equal(processSpec))
				Ω(calledProcessIO).Should(Equal(processIO))
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
						Ω(process.Wait()).Should(Equal(42))

						Ω(innerConnection.AttachCallCount()).Should(Equal(1))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
						Ω(handle).Should(Equal("la-contineur"))
						Ω(processID).Should(Equal(uint32(6)))
						Ω(calledProcessIO).Should(Equal(processIO))
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
						Ω(process.Signal(garden.SignalTerminate)).Should(Succeed())
						Ω(fakeProcess.SignalArgsForCall(0)).Should(Equal(garden.SignalTerminate))

						Ω(innerConnection.AttachCallCount()).Should(Equal(1))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
						Ω(handle).Should(Equal("la-contineur"))
						Ω(processID).Should(Equal(uint32(6)))
						Ω(calledProcessIO).Should(Equal(processIO))
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

						Ω(process.SetTTY(ttySpec)).Should(Succeed())
						Ω(fakeProcess.SetTTYArgsForCall(0)).Should(Equal(ttySpec))

						Ω(innerConnection.AttachCallCount()).Should(Equal(1))
						handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
						Ω(handle).Should(Equal("la-contineur"))
						Ω(processID).Should(Equal(uint32(6)))
						Ω(calledProcessIO).Should(Equal(processIO))
					})
				})
			})
		})
	})
})

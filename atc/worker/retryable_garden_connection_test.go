package worker_test

import (
	"fmt"
	"io"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client/connection/connectionfakes"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Retryable Garden Connection", func() {
	var innerConnection *connectionfakes.FakeConnection
	var conn *worker.RetryableConnection

	BeforeEach(func() {
		innerConnection = new(connectionfakes.FakeConnection)
		conn = worker.NewRetryableConnection(innerConnection)
	})

	Describe("StreamIn", func() {
		var spec garden.StreamInSpec

		BeforeEach(func() {
			spec = garden.StreamInSpec{
				Path: "some-path",
				User: "some-user",
			}
			innerConnection.StreamInReturns(nil)
			err := conn.StreamIn("some-handle", spec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.StreamInCallCount()).To(Equal(1))
			calledHandle, calledSpec := innerConnection.StreamInArgsForCall(0)
			Expect(calledHandle).To(Equal("some-handle"))
			Expect(calledSpec).To(Equal(spec))
		})
	})

	Describe("Capacity", func() {
		var capacity garden.Capacity
		var gotCapacity garden.Capacity
		var err error

		BeforeEach(func() {
			capacity = garden.Capacity{MemoryInBytes: 1024}
			innerConnection.CapacityReturns(capacity, nil)
			gotCapacity, err = conn.Capacity()
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.CapacityCallCount()).To(Equal(1))
			Expect(gotCapacity).To(Equal(capacity))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Create", func() {
		spec := garden.ContainerSpec{
			RootFSPath: "/dev/mouse",
		}

		var gotHandle string
		var err error

		BeforeEach(func() {
			innerConnection.CreateReturns("some-handle", nil)
			gotHandle, err = conn.Create(spec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.CreateCallCount()).To(Equal(1))
			calledSpec := innerConnection.CreateArgsForCall(0)
			Expect(calledSpec).To(Equal(spec))
			Expect(gotHandle).To(Equal("some-handle"))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Destroy", func() {
		BeforeEach(func() {
			innerConnection.DestroyReturns(nil)
			err := conn.Destroy("some-handle")
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.DestroyCallCount()).To(Equal(1))
			calledHandle := innerConnection.DestroyArgsForCall(0)
			Expect(calledHandle).To(Equal("some-handle"))
		})
	})

	Describe("Stop", func() {
		BeforeEach(func() {
			innerConnection.StopReturns(nil)
			err := conn.Stop("some-handle", true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.StopCallCount()).To(Equal(1))
			calledHandle, kill := innerConnection.StopArgsForCall(0)
			Expect(calledHandle).To(Equal("some-handle"))
			Expect(kill).To(BeTrue())
		})
	})

	Describe("CurrentBandwidthLimits", func() {

		handle := "suitcase"

		limits := garden.BandwidthLimits{
			RateInBytesPerSecond: 234,
		}

		var gotLimits garden.BandwidthLimits
		var err error

		Context("CurrentBandwidthLimits succeeds", func() {
			BeforeEach(func() {
				innerConnection.CurrentBandwidthLimitsReturns(limits, err)
				gotLimits, err = conn.CurrentBandwidthLimits(handle)
				Expect(err).NotTo(HaveOccurred())
			})

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
		var err error

		Context("CurrentCPULimits succeeds", func() {
			BeforeEach(func() {
				innerConnection.CurrentCPULimitsReturns(limits, err)
				gotLimits, err = conn.CurrentCPULimits(handle)
				Expect(err).NotTo(HaveOccurred())
			})

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

	Context("CurrentDiskLimits succeeds", func() {
		handle := "suitcase"

		limits := garden.DiskLimits{
			ByteHard: 234,
		}

		var gotLimits garden.DiskLimits
		var err error

		BeforeEach(func() {
			innerConnection.CurrentDiskLimitsReturns(limits, err)
			gotLimits, err = conn.CurrentDiskLimits(handle)
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.CurrentDiskLimitsCallCount()).To(Equal(1))

			calledHandle := innerConnection.CurrentDiskLimitsArgsForCall(0)
			Expect(calledHandle).To(Equal(handle))
		})

		It("returns the limits", func() {
			Expect(gotLimits).To(Equal(limits))
		})
	})

	Describe("CurrentMemoryLimits", func() {
		handle := "suitcase"

		limits := garden.MemoryLimits{
			LimitInBytes: 234,
		}

		var gotLimits garden.MemoryLimits
		var err error

		Context("CurrentMemoryLimits succeeds", func() {
			BeforeEach(func() {
				innerConnection.CurrentMemoryLimitsReturns(limits, err)
				gotLimits, err = conn.CurrentMemoryLimits(handle)
				Expect(err).NotTo(HaveOccurred())
			})

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

	Describe("Property", func() {
		handle := "suitcase"
		property := "dfghjkl"

		var gotValue string
		var err error

		BeforeEach(func() {
			innerConnection.PropertyReturns("some-value", err)
			gotValue, err = conn.Property(handle, property)
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.PropertyCallCount()).To(Equal(1))

			calledHandle, calledProperty := innerConnection.PropertyArgsForCall(0)
			Expect(calledHandle).To(Equal(handle))
			Expect(calledProperty).To(Equal(property))
		})

		It("returns the value", func() {
			Expect(gotValue).To(Equal("some-value"))
		})
	})

	Describe("StreamOut", func() {
		var spec garden.StreamOutSpec

		BeforeEach(func() {
			spec = garden.StreamOutSpec{
				Path: "/etc/passwd",
				User: "admin",
			}

			innerConnection.StreamOutReturns(gbytes.NewBuffer(), nil)

			_, err := conn.StreamOut("some-handle", spec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.StreamOutCallCount()).To(Equal(1))
			calledHandle, calledSpec := innerConnection.StreamOutArgsForCall(0)
			Expect(calledHandle).To(Equal("some-handle"))
			Expect(calledSpec).To(Equal(spec))
		})
	})

	Describe("Attach", func() {
		var (
			fakeProcess *gardenfakes.FakeProcess
			process     garden.Process
		)

		processIO := garden.ProcessIO{
			Stdout: gbytes.NewBuffer(),
		}

		BeforeEach(func() {
			fakeProcess = new(gardenfakes.FakeProcess)
			fakeProcess.IDReturns("process-id")
			innerConnection.AttachReturns(fakeProcess, nil)
			var err error
			process, err = conn.Attach("la-contineur", "process-id", processIO)
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls through to garden", func() {
			Expect(innerConnection.AttachCallCount()).To(Equal(1))

			handle, processID, calledProcessIO := innerConnection.AttachArgsForCall(0)
			Expect(handle).To(Equal("la-contineur"))
			Expect(processID).To(Equal("process-id"))
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
					Expect(processID).To(Equal("process-id"))
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
					Expect(processID).To(Equal("process-id"))
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
					Expect(processID).To(Equal("process-id"))
					Expect(calledProcessIO).To(Equal(processIO))
				})
			})
		})
	})

	Describe("Run", func() {
		var (
			fakeProcess *gardenfakes.FakeProcess
			process     garden.Process
		)

		processSpec := garden.ProcessSpec{
			Path: "reboot",
		}

		processIO := garden.ProcessIO{
			Stdout: gbytes.NewBuffer(),
		}

		BeforeEach(func() {
			fakeProcess = new(gardenfakes.FakeProcess)
			fakeProcess.IDReturns("process-id")
			innerConnection.RunReturns(fakeProcess, nil)
			var err error
			process, err = conn.Run("la-contineur", processSpec, processIO)
			Expect(err).NotTo(HaveOccurred())
		})

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
					Expect(processID).To(Equal("process-id"))
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
					Expect(processID).To(Equal("process-id"))
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
					Expect(processID).To(Equal("process-id"))
					Expect(calledProcessIO).To(Equal(processIO))
				})
			})
		})
	})
})

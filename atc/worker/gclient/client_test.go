package gclient_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/atc/worker/gclient/connection/connectionfakes"
)

var _ = Describe("Client", func() {
	var client gclient.Client

	var fakeConnection *connectionfakes.FakeConnection

	BeforeEach(func() {
		fakeConnection = new(connectionfakes.FakeConnection)
	})

	JustBeforeEach(func() {
		client = gclient.NewClient(fakeConnection)
	})

	Describe("Capacity", func() {
		BeforeEach(func() {
			fakeConnection.CapacityReturns(
				garden.Capacity{
					MemoryInBytes: 1111,
					DiskInBytes:   2222,
					MaxContainers: 42,
				},
				nil,
			)
		})

		It("sends a capacity request and returns the capacity", func() {
			capacity, err := client.Capacity()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(capacity.MemoryInBytes).Should(Equal(uint64(1111)))
			Ω(capacity.DiskInBytes).Should(Equal(uint64(2222)))
		})

		Context("when getting capacity fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.CapacityReturns(garden.Capacity{}, disaster)
			})

			It("returns the error", func() {
				_, err := client.Capacity()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("BulkInfo", func() {
		expectedBulkInfo := map[string]garden.ContainerInfoEntry{
			"handle1": garden.ContainerInfoEntry{
				Info: garden.ContainerInfo{
					State: "container1State",
				},
			},
			"handle2": garden.ContainerInfoEntry{
				Info: garden.ContainerInfo{
					State: "container1State",
				},
			},
		}
		handles := []string{"handle1", "handle2"}

		It("gets info for the requested containers", func() {
			fakeConnection.BulkInfoReturns(expectedBulkInfo, nil)

			bulkInfo, err := client.BulkInfo(handles)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.BulkInfoCallCount()).Should(Equal(1))
			Ω(fakeConnection.BulkInfoArgsForCall(0)).Should(Equal(handles))
			Ω(bulkInfo).Should(Equal(expectedBulkInfo))
		})

		Context("when there is a error with the connection", func() {
			expectedBulkInfo := map[string]garden.ContainerInfoEntry{}

			BeforeEach(func() {
				fakeConnection.BulkInfoReturns(expectedBulkInfo, errors.New("Oh noes!"))
			})

			It("returns the error", func() {
				_, err := client.BulkInfo(handles)
				Ω(err).Should(MatchError("Oh noes!"))
			})
		})
	})

	Describe("BulkMetrics", func() {
		expectedBulkMetrics := map[string]garden.ContainerMetricsEntry{
			"handle1": garden.ContainerMetricsEntry{
				Metrics: garden.Metrics{
					DiskStat: garden.ContainerDiskStat{
						TotalInodesUsed:     1,
						TotalBytesUsed:      2,
						ExclusiveBytesUsed:  3,
						ExclusiveInodesUsed: 4,
					},
				},
			},
			"handle2": garden.ContainerMetricsEntry{
				Metrics: garden.Metrics{
					DiskStat: garden.ContainerDiskStat{
						TotalInodesUsed:     5,
						TotalBytesUsed:      6,
						ExclusiveBytesUsed:  7,
						ExclusiveInodesUsed: 8,
					},
				},
			},
		}
		handles := []string{"handle1", "handle2"}

		It("gets info for the requested containers", func() {
			fakeConnection.BulkMetricsReturns(expectedBulkMetrics, nil)

			bulkInfo, err := client.BulkMetrics(handles)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.BulkMetricsCallCount()).Should(Equal(1))
			Ω(fakeConnection.BulkMetricsArgsForCall(0)).Should(Equal(handles))
			Ω(bulkInfo).Should(Equal(expectedBulkMetrics))
		})

		Context("when there is a error with the connection", func() {
			expectedBulkMetrics := map[string]garden.ContainerMetricsEntry{}

			BeforeEach(func() {
				fakeConnection.BulkMetricsReturns(expectedBulkMetrics, errors.New("Oh noes!"))
			})

			It("returns the error", func() {
				_, err := client.BulkMetrics(handles)
				Ω(err).Should(MatchError("Oh noes!"))
			})
		})
	})

	Describe("Create", func() {
		It("sends a create request and returns a container", func() {
			spec := garden.ContainerSpec{
				RootFSPath: "/some/roofs",
			}

			fakeConnection.CreateReturns("some-handle", nil)

			container, err := client.Create(spec)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(container).ShouldNot(BeNil())

			actualSpec := fakeConnection.CreateArgsForCall(0)
			Ω(actualSpec).Should(Equal(spec))

			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.CreateReturns("", disaster)
			})

			It("returns it", func() {
				_, err := client.Create(garden.ContainerSpec{})
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Containers", func() {
		It("sends a list request and returns all containers", func() {
			fakeConnection.ListReturns([]string{"handle-a", "handle-b"}, nil)

			props := garden.Properties{"foo": "bar"}

			containers, err := client.Containers(props)
			Ω(err).ShouldNot(HaveOccurred())

			actualProps := fakeConnection.ListArgsForCall(0)
			Ω(actualProps).Should(Equal(props))

			Ω(containers).Should(HaveLen(2))
			Ω(containers[0].Handle()).Should(Equal("handle-a"))
			Ω(containers[1].Handle()).Should(Equal("handle-b"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.ListReturns(nil, disaster)
			})

			It("returns it", func() {
				_, err := client.Containers(nil)
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Destroy", func() {
		It("sends a destroy request", func() {
			err := client.Destroy("some-handle")
			Ω(err).ShouldNot(HaveOccurred())
			actualHandle := fakeConnection.DestroyArgsForCall(0)
			Ω(actualHandle).Should(Equal("some-handle"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.DestroyReturns(disaster)
			})

			It("returns it", func() {
				err := client.Destroy("some-handle")
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Lookup", func() {
		It("sends a list request", func() {
			fakeConnection.ListReturns([]string{"some-handle", "some-other-handle"}, nil)

			container, err := client.Lookup("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				fakeConnection.ListReturns([]string{"some-other-handle"}, nil)
			})

			It("returns ContainerNotFoundError", func() {
				_, err := client.Lookup("some-handle")
				Ω(err).Should(MatchError(garden.ContainerNotFoundError{Handle: "some-handle"}))
			})
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.ListReturns(nil, disaster)
			})

			It("returns it", func() {
				_, err := client.Lookup("some-handle")
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})

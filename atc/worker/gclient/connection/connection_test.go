package connection_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/concourse/atc/worker/gclient/connection"
	"github.com/concourse/concourse/atc/worker/gclient/connection/connectionfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/transport"
)

var _ = Describe("Connection", func() {
	var (
		connection     Connection
		resourceLimits garden.ResourceLimits
		server         *ghttp.Server
		hijacker       HijackStreamer
		network        string
		address        string
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
		network = "tcp"
		address = server.HTTPTestServer.Listener.Addr().String()
		hijacker = NewHijackStreamer(network, address)
	})

	JustBeforeEach(func() {
		connection = NewWithHijacker(hijacker, lagertest.NewTestLogger("test-connection"))
	})

	BeforeEach(func() {
		rlimits := &garden.ResourceLimits{
			As:         uint64ptr(1),
			Core:       uint64ptr(2),
			Cpu:        uint64ptr(4),
			Data:       uint64ptr(5),
			Fsize:      uint64ptr(6),
			Locks:      uint64ptr(7),
			Memlock:    uint64ptr(8),
			Msgqueue:   uint64ptr(9),
			Nice:       uint64ptr(10),
			Nofile:     uint64ptr(11),
			Nproc:      uint64ptr(12),
			Rss:        uint64ptr(13),
			Rtprio:     uint64ptr(14),
			Sigpending: uint64ptr(15),
			Stack:      uint64ptr(16),
		}

		resourceLimits = garden.ResourceLimits{
			As:         rlimits.As,
			Core:       rlimits.Core,
			Cpu:        rlimits.Cpu,
			Data:       rlimits.Data,
			Fsize:      rlimits.Fsize,
			Locks:      rlimits.Locks,
			Memlock:    rlimits.Memlock,
			Msgqueue:   rlimits.Msgqueue,
			Nice:       rlimits.Nice,
			Nofile:     rlimits.Nofile,
			Nproc:      rlimits.Nproc,
			Rss:        rlimits.Rss,
			Rtprio:     rlimits.Rtprio,
			Sigpending: rlimits.Sigpending,
			Stack:      rlimits.Stack,
		}
	})

	Describe("Ping", func() {
		Context("when the response is successful", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(200, "{}"),
					),
				)
			})

			It("should ping the server", func() {
				err := connection.Ping()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("should return an error", func() {
				err := connection.Ping()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the request fails with special error code http.StatusServiceUnavailable", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(http.StatusGatewayTimeout, `{ "Type": "ServiceUnavailableError" , "Message": "Special Error Message"}`),
					),
				)
			})

			It("should return an error without the http info in the error message", func() {
				err := connection.Ping()
				Expect(err).To(MatchError("Special Error Message"))
			})

			It("should return an error of the appropriate type", func() {
				err := connection.Ping()
				Expect(err).To(BeAssignableToTypeOf(garden.ServiceUnavailableError{}))
			})
		})

		Context("when the request fails with extra special error code http.StatusInternalServerError", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(http.StatusGatewayTimeout, `{ "Type": "UnrecoverableError" , "Message": "Extra Special Error Message"}`),
					),
				)
			})

			It("should return an error without the http info in the error message", func() {
				err := connection.Ping()
				Expect(err).To(MatchError("Extra Special Error Message"))
			})

			It("should return an error of the appropriate unrecoverable type", func() {
				err := connection.Ping()
				Expect(err).To(BeAssignableToTypeOf(garden.UnrecoverableError{}))
			})
		})
	})

	Describe("Getting capacity", func() {
		Context("when the response is successful", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/capacity"),
						ghttp.RespondWith(200, marshalProto(&garden.Capacity{
							MemoryInBytes: 1111,
							DiskInBytes:   2222,
							MaxContainers: 42,
						}))))
			})

			It("should return the server's capacity", func() {
				capacity, err := connection.Capacity()
				Expect(err).ToNot(HaveOccurred())

				Expect(capacity.MemoryInBytes).To(BeNumerically("==", 1111))
				Expect(capacity.DiskInBytes).To(BeNumerically("==", 2222))
				Expect(capacity.MaxContainers).To(BeNumerically("==", 42))
			})
		})

		Context("when the request fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/capacity"),
						ghttp.RespondWith(500, "")))
			})

			It("should return an error", func() {
				_, err := connection.Capacity()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Creating", func() {
		var spec garden.ContainerSpec

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/containers"),
					verifyRequestBody(&spec, &garden.ContainerSpec{}),
					ghttp.RespondWith(200, marshalProto(&struct{ Handle string }{"foohandle"}))))
		})

		Context("with an empty ContainerSpec", func() {
			BeforeEach(func() {
				spec = garden.ContainerSpec{}
			})

			It("sends the ContainerSpec over the connection as JSON", func() {
				handle, err := connection.Create(spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(handle).To(Equal("foohandle"))
			})
		})

		Context("with a fully specified ContainerSpec", func() {
			BeforeEach(func() {
				spec = garden.ContainerSpec{
					Handle:     "some-handle",
					GraceTime:  10 * time.Second,
					RootFSPath: "some-rootfs-path",
					Network:    "some-network",
					BindMounts: []garden.BindMount{
						{
							SrcPath: "/src-a",
							DstPath: "/dst-a",
							Mode:    garden.BindMountModeRO,
							Origin:  garden.BindMountOriginHost,
						},
						{
							SrcPath: "/src-b",
							DstPath: "/dst-b",
							Mode:    garden.BindMountModeRW,
							Origin:  garden.BindMountOriginContainer,
						},
					},
					Properties: map[string]string{
						"foo": "bar",
					},
					Env: []string{"env1=env1Value1"},
				}
			})

			It("sends the ContainerSpec over the connection as JSON", func() {
				handle, err := connection.Create(spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(handle).To(Equal("foohandle"))
			})
		})
	})

	Describe("Destroying", func() {
		Context("when destroying succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/foo"),
						ghttp.RespondWith(200, "{}")))
			})

			It("should stop the container", func() {
				err := connection.Destroy("foo")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when destroying fails because the container doesn't exist", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/foo"),
						ghttp.RespondWith(404, `{ "Type": "ContainerNotFoundError", "Handle" : "some handle"}`)))
			})

			It("return an appropriate error with the message", func() {
				err := connection.Destroy("foo")
				Expect(err).To(MatchError(garden.ContainerNotFoundError{Handle: "some handle"}))
			})
		})
	})

	Describe("Stopping", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/containers/foo/stop"),
					verifyRequestBody(map[string]interface{}{
						"kill": true,
					}, make(map[string]interface{})),
					ghttp.RespondWith(200, "{}")))
		})

		It("should stop the container", func() {
			err := connection.Stop("foo", true)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("fetching limit info", func() {
		Describe("getting memory limits", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/memory"),
						ghttp.RespondWith(200, marshalProto(&garden.MemoryLimits{
							LimitInBytes: 40,
						}, &garden.MemoryLimits{})),
					),
				)
			})

			It("gets the memory limit", func() {
				currentLimits, err := connection.CurrentMemoryLimits("foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(currentLimits.LimitInBytes).To(BeNumerically("==", 40))
			})
		})

		Describe("getting cpu limits", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/cpu"),
						ghttp.RespondWith(200, marshalProto(&garden.CPULimits{
							LimitInShares: 40,
						})),
					),
				)
			})

			It("gets the cpu limit", func() {
				limits, err := connection.CurrentCPULimits("foo")
				Expect(err).ToNot(HaveOccurred())

				Expect(limits.LimitInShares).To(BeNumerically("==", 40))
			})
		})

		Describe("getting bandwidth limits", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/bandwidth"),
						ghttp.RespondWith(200, marshalProto(&garden.BandwidthLimits{
							RateInBytesPerSecond:      1,
							BurstRateInBytesPerSecond: 2,
						})),
					),
				)
			})

			It("gets the bandwidth limit", func() {
				limits, err := connection.CurrentBandwidthLimits("foo")
				Expect(err).ToNot(HaveOccurred())

				Expect(limits.RateInBytesPerSecond).To(BeNumerically("==", 1))
				Expect(limits.BurstRateInBytesPerSecond).To(BeNumerically("==", 2))
			})
		})
	})

	Describe("NetIn", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/containers/foo-handle/net/in"),
					verifyRequestBody(map[string]interface{}{
						"handle":         "foo-handle",
						"host_port":      float64(8080),
						"container_port": float64(8081),
					}, make(map[string]interface{})),
					ghttp.RespondWith(200, marshalProto(map[string]interface{}{
						"host_port":      1234,
						"container_port": 1235,
					}))))
		})

		It("should return the allocated ports", func() {
			hostPort, containerPort, err := connection.NetIn("foo-handle", 8080, 8081)
			Expect(err).ToNot(HaveOccurred())
			Expect(hostPort).To(Equal(uint32(1234)))
			Expect(containerPort).To(Equal(uint32(1235)))
		})
	})

	Describe("NetOut", func() {
		var (
			rule   garden.NetOutRule
			handle string
		)

		BeforeEach(func() {
			handle = "foo-handle"
		})

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", fmt.Sprintf("/containers/%s/net/out", handle)),
					verifyRequestBody(&rule, &garden.NetOutRule{}),
					ghttp.RespondWith(200, "{}")))
		})

		Context("when a NetOutRule is passed", func() {
			BeforeEach(func() {
				rule = garden.NetOutRule{
					Protocol: garden.ProtocolICMP,
					Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("1.2.3.4"))},
					Ports:    []garden.PortRange{garden.PortRangeFromPort(2), garden.PortRangeFromPort(4)},
					ICMPs:    &garden.ICMPControl{Type: 3, Code: garden.ICMPControlCode(3)},
					Log:      true,
				}
			})

			It("should send the rule over the wire", func() {
				Expect(connection.NetOut(handle, rule)).To(Succeed())
			})
		})
	})

	Describe("Listing containers", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/containers", "foo=bar"),
					ghttp.RespondWith(200, marshalProto(&struct {
						Handles []string `json:"handles"`
					}{
						[]string{"container1", "container2", "container3"},
					}))))
		})

		It("should return the list of containers", func() {
			handles, err := connection.List(map[string]string{"foo": "bar"})

			Expect(err).ToNot(HaveOccurred())
			Expect(handles).To(Equal([]string{"container1", "container2", "container3"}))
		})
	})

	Describe("Getting container properties", func() {
		handle := "container-handle"
		var status int

		BeforeEach(func() {
			status = 200
		})

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", fmt.Sprintf("/containers/%s/properties", handle)),
					ghttp.RespondWith(status, "{\"foo\": \"bar\"}")))
		})

		It("returns the map of properties", func() {
			properties, err := connection.Properties(handle)

			Expect(err).ToNot(HaveOccurred())
			Expect(properties).To(
				Equal(garden.Properties{
					"foo": "bar",
				}),
			)
		})

		Context("when getting container properties fails", func() {
			BeforeEach(func() {
				status = 400
			})

			It("returns an error", func() {
				_, err := connection.Properties(handle)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Get container property", func() {

		handle := "container-handle"
		propertyName := "property_name"
		propertyValue := "property_value"
		var status int

		BeforeEach(func() {
			status = 200
		})

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", fmt.Sprintf("/containers/%s/properties/%s", handle, propertyName)),
					ghttp.RespondWith(status, fmt.Sprintf("{\"value\": \"%s\"}", propertyValue))))
		})

		It("returns the property", func() {
			property, err := connection.Property(handle, propertyName)

			Expect(err).ToNot(HaveOccurred())
			Expect(property).To(Equal(propertyValue))
		})

		Context("when getting container property fails", func() {
			BeforeEach(func() {
				status = 400
			})

			It("returns an error", func() {
				_, err := connection.Property(handle, propertyName)
				Expect(err).To(HaveOccurred())
			})
		})

	})

	Describe("Getting container metrics", func() {
		handle := "container-handle"
		metrics := garden.Metrics{
			MemoryStat: garden.ContainerMemoryStat{
				Cache:                   1,
				Rss:                     2,
				MappedFile:              3,
				Pgpgin:                  4,
				Pgpgout:                 5,
				Swap:                    6,
				Pgfault:                 7,
				Pgmajfault:              8,
				InactiveAnon:            9,
				ActiveAnon:              10,
				InactiveFile:            11,
				ActiveFile:              12,
				Unevictable:             13,
				HierarchicalMemoryLimit: 14,
				HierarchicalMemswLimit:  15,
				TotalCache:              16,
				TotalRss:                17,
				TotalMappedFile:         18,
				TotalPgpgin:             19,
				TotalPgpgout:            20,
				TotalSwap:               21,
				TotalPgfault:            22,
				TotalPgmajfault:         23,
				TotalInactiveAnon:       24,
				TotalActiveAnon:         25,
				TotalInactiveFile:       26,
				TotalActiveFile:         27,
				TotalUnevictable:        28,
				TotalUsageTowardLimit:   7, // TotalRss+(TotalCache-TotalInactiveFile)
			},
			CPUStat: garden.ContainerCPUStat{
				Usage:  1,
				User:   2,
				System: 3,
			},

			DiskStat: garden.ContainerDiskStat{
				TotalBytesUsed:      11,
				TotalInodesUsed:     12,
				ExclusiveBytesUsed:  13,
				ExclusiveInodesUsed: 14,
			},
		}
		var status int

		BeforeEach(func() {
			status = 200
		})

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", fmt.Sprintf("/containers/%s/metrics", handle)),
					ghttp.RespondWith(status, marshalProto(metrics))))
		})

		It("returns the MemoryStat, CPUStat and DiskStat", func() {
			returnedMetrics, err := connection.Metrics(handle)

			Expect(err).ToNot(HaveOccurred())
			Expect(returnedMetrics).To(Equal(metrics))
		})

		Context("when getting container metrics fails", func() {
			BeforeEach(func() {
				status = 400
			})

			It("returns an error", func() {
				_, err := connection.Metrics(handle)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Setting the grace time", func() {
		var (
			status    int
			handle    string
			graceTime time.Duration
		)

		BeforeEach(func() {
			handle = "container-handle"
			graceTime = time.Second * 5
			status = 200
		})

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", fmt.Sprintf("/containers/%s/grace_time", handle)),
					// interface{} confusion: the JSON decoder decodes numberics to float64...
					verifyRequestBody(float64(graceTime), float64(0)),
					ghttp.RespondWith(status, "{}"),
				),
			)
		})

		It("send SetGraceTime request", func() {
			Expect(connection.SetGraceTime(handle, graceTime)).To(Succeed())
		})

		Context("when setting grace time fails", func() {
			BeforeEach(func() {
				status = 400
			})

			It("returns an error", func() {
				Expect(connection.SetGraceTime(handle, graceTime)).ToNot(Succeed())
			})
		})
	})

	Describe("Getting container info", func() {
		var infoResponse garden.ContainerInfo

		JustBeforeEach(func() {
			infoResponse = garden.ContainerInfo{
				State: "chilling out",
				Events: []string{
					"maxing",
					"relaxing all cool",
				},
				HostIP:        "host-ip",
				ContainerIP:   "container-ip",
				ContainerPath: "container-path",
				ProcessIDs:    []string{"process-handle-1", "process-handle-2"},
				Properties: garden.Properties{
					"prop-key": "prop-value",
				},
				MappedPorts: []garden.PortMapping{
					{HostPort: 1234, ContainerPort: 5678},
					{HostPort: 1235, ContainerPort: 5679},
				},
			}

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/containers/some-handle/info"),
					ghttp.RespondWith(200, marshalProto(infoResponse))))
		})

		It("should return the container's info", func() {
			info, err := connection.Info("some-handle")
			Expect(err).ToNot(HaveOccurred())

			Expect(info).To(Equal(infoResponse))
		})
	})

	Describe("BulkInfo", func() {

		expectedBulkInfo := map[string]garden.ContainerInfoEntry{
			"handle1": garden.ContainerInfoEntry{
				Info: garden.ContainerInfo{
					State: "container1state",
				},
			},
			"handle2": garden.ContainerInfoEntry{
				Info: garden.ContainerInfo{
					State: "container2state",
				},
			},
		}

		handles := []string{"handle1", "handle2"}
		queryParams := "handles=" + strings.Join(handles, "%2C")

		Context("when the response is successful", func() {
			JustBeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/bulk_info", queryParams),
						ghttp.RespondWith(200, marshalProto(expectedBulkInfo))))
			})

			It("returns info about containers", func() {
				bulkInfo, err := connection.BulkInfo(handles)
				Expect(err).ToNot(HaveOccurred())
				Expect(bulkInfo).To(Equal(expectedBulkInfo))
			})
		})

		Context("when the request fails", func() {
			JustBeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/bulk_info", queryParams),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("returns the error", func() {
				_, err := connection.BulkInfo(handles)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when a container is in error state", func() {
			It("returns the error for the container", func() {

				expectedBulkInfo := map[string]garden.ContainerInfoEntry{
					"error": garden.ContainerInfoEntry{
						Err: &garden.Error{Err: errors.New("Oopps")},
					},
					"success": garden.ContainerInfoEntry{
						Info: garden.ContainerInfo{
							State: "container2state",
						},
					},
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/bulk_info", queryParams),
						ghttp.RespondWith(200, marshalProto(expectedBulkInfo))))

				bulkInfo, err := connection.BulkInfo(handles)
				Expect(err).ToNot(HaveOccurred())
				Expect(bulkInfo).To(Equal(expectedBulkInfo))
			})
		})
	})

	Describe("BulkMetrics", func() {

		expectedBulkMetrics := map[string]garden.ContainerMetricsEntry{
			"handle1": garden.ContainerMetricsEntry{
				Metrics: garden.Metrics{
					DiskStat: garden.ContainerDiskStat{
						TotalInodesUsed:     4,
						TotalBytesUsed:      3,
						ExclusiveBytesUsed:  2,
						ExclusiveInodesUsed: 1,
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
		queryParams := "handles=" + strings.Join(handles, "%2C")

		Context("when the response is successful", func() {
			JustBeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/bulk_metrics", queryParams),
						ghttp.RespondWith(200, marshalProto(expectedBulkMetrics))))
			})

			It("returns info about containers", func() {
				bulkMetrics, err := connection.BulkMetrics(handles)
				Expect(err).ToNot(HaveOccurred())
				Expect(bulkMetrics).To(Equal(expectedBulkMetrics))
			})
		})

		Context("when the request fails", func() {
			JustBeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/bulk_metrics", queryParams),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("returns the error", func() {
				_, err := connection.BulkMetrics(handles)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when a container has an error", func() {
			It("returns the error for the container", func() {

				errorBulkMetrics := map[string]garden.ContainerMetricsEntry{
					"error": garden.ContainerMetricsEntry{
						Err: &garden.Error{Err: errors.New("Oh noes!")},
					},
					"success": garden.ContainerMetricsEntry{
						Metrics: garden.Metrics{
							DiskStat: garden.ContainerDiskStat{
								TotalInodesUsed: 1,
							},
						},
					},
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/bulk_metrics", queryParams),
						ghttp.RespondWith(200, marshalProto(errorBulkMetrics))))

				bulkMetrics, err := connection.BulkMetrics(handles)
				Expect(err).ToNot(HaveOccurred())
				Expect(bulkMetrics).To(Equal(errorBulkMetrics))
			})
		})
	})

	Describe("Streaming in", func() {
		Context("when streaming in succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "user=alice&destination=%2Fbar"),
						func(w http.ResponseWriter, r *http.Request) {
							body, err := ioutil.ReadAll(r.Body)
							Expect(err).ToNot(HaveOccurred())

							Expect(string(body)).To(Equal("chunk-1chunk-2"))
						},
					),
				)
			})

			It("tells garden.to stream, and then streams the content as a series of chunks", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")

				err := connection.StreamIn("foo-handle", garden.StreamInSpec{User: "alice", Path: "/bar", TarStream: buffer})
				Expect(err).ToNot(HaveOccurred())

				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when streaming in returns an error response", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "user=bob&destination=%2Fbar"),
						ghttp.RespondWith(http.StatusInternalServerError, "no."),
					),
				)
			})

			It("returns an error on close", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")
				err := connection.StreamIn("foo-handle", garden.StreamInSpec{User: "bob", Path: "/bar", TarStream: buffer})
				Expect(err).To(HaveOccurred())

				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when streaming in fails hard", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "user=bob&destination=%2Fbar"),
						ghttp.RespondWith(http.StatusInternalServerError, "no."),
						func(w http.ResponseWriter, r *http.Request) {
							server.CloseClientConnections()
						},
					),
				)
			})

			It("returns an error on close", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")

				err := connection.StreamIn("foo-handle", garden.StreamInSpec{User: "bob", Path: "/bar", TarStream: buffer})
				Expect(err).To(HaveOccurred())

				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})
		})
	})

	Describe("Streaming Out", func() {
		Context("when streaming succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/files", "user=frank&source=%2Fbar"),
						ghttp.RespondWith(200, "hello-world!"),
					),
				)
			})

			It("asks garden.for the given file, then reads its content", func() {
				reader, err := connection.StreamOut("foo-handle", garden.StreamOutSpec{User: "frank", Path: "/bar"})
				Expect(err).ToNot(HaveOccurred())

				readBytes, err := ioutil.ReadAll(reader)
				Expect(err).ToNot(HaveOccurred())
				Expect(readBytes).To(Equal([]byte("hello-world!")))

				reader.Close()
			})
		})

		Context("when streaming fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/files", "user=deandra&source=%2Fbar"),
						func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("Content-Length", "500")
						},
					),
				)
			})

			It("asks garden.for the given file, then reads its content", func() {
				reader, err := connection.StreamOut("foo-handle", garden.StreamOutSpec{User: "deandra", Path: "/bar"})
				Expect(err).ToNot(HaveOccurred())

				_, err = ioutil.ReadAll(reader)
				reader.Close()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Running", func() {
		var (
			spec         garden.ProcessSpec
			stdInContent chan string
		)

		Context("when streaming succeeds to completion", func() {
			BeforeEach(func() {
				spec = garden.ProcessSpec{
					Path:   "lol",
					Args:   []string{"arg1", "arg2"},
					Dir:    "/some/dir",
					User:   "root",
					Limits: resourceLimits,
				}
				stdInContent = make(chan string)

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.VerifyJSONRepresenting(spec),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Expect(err).ToNot(HaveOccurred())

							Expect(payload).To(Equal(map[string]interface{}{
								"process_id": "process-handle",
								"source":     float64(transport.Stdin),
								"data":       "stdin data",
							}))
							stdInContent <- payload["data"].(string)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  "process-handle",
								"exit_status": 3,
							})
						},
					),
					stdoutStream("foo-handle", "process-handle", 123, func(conn net.Conn) {
						conn.Write([]byte("stdout data"))
						conn.Write([]byte(fmt.Sprintf("roundtripped %s", <-stdInContent)))
					}),
					stderrStream("foo-handle", "process-handle", 123, func(conn net.Conn) {
						conn.Write([]byte("stderr data"))
					}),
				)
			})

			It("streams the data, closes the destinations, and notifies of exit", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()

				process, err := connection.Run(context.TODO(), "foo-handle", spec, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: stdout,
					Stderr: stderr,
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(process.ID()).To(Equal("process-handle"))

				Eventually(stdout).Should(gbytes.Say("stdout data"))
				Eventually(stdout).Should(gbytes.Say("roundtripped stdin data"))
				Eventually(stderr).Should(gbytes.Say("stderr data"))

				status, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(3))
			})

			It("finishes streaming stdout and stderr before returning from .Wait", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()

				process, err := connection.Run(context.TODO(), "foo-handle", spec, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: stdout,
					Stderr: stderr,
				})
				Expect(err).ToNot(HaveOccurred())

				process.Wait()
				Expect(stdout).To(gbytes.Say("roundtripped stdin data"))
				Expect(stderr).To(gbytes.Say("stderr data"))
			})

			Describe("connection leak avoidance", func() {
				var fakeHijacker *connectionfakes.FakeHijackStreamer
				var wrappedConnections []*wrappedConnection

				BeforeEach(func() {
					wrappedConnections = []*wrappedConnection{}
					netHijacker := hijacker
					fakeHijacker = new(connectionfakes.FakeHijackStreamer)
					fakeHijacker.HijackStub = func(ctx context.Context, handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (net.Conn, *bufio.Reader, error) {
						conn, resp, err := netHijacker.Hijack(ctx, handler, body, params, query, contentType)
						wc := &wrappedConnection{Conn: conn}
						wrappedConnections = append(wrappedConnections, wc)
						return wc, resp, err
					}

					hijacker = fakeHijacker
				})

				It("should not leak net.Conn from Run", func() {
					stdout := gbytes.NewBuffer()
					stderr := gbytes.NewBuffer()

					process, err := connection.Run(context.TODO(), "foo-handle", spec, garden.ProcessIO{
						Stdin:  bytes.NewBufferString("stdin data"),
						Stdout: stdout,
						Stderr: stderr,
					})
					Expect(err).ToNot(HaveOccurred())

					process.Wait()
					Expect(stdout).To(gbytes.Say("roundtripped stdin data"))
					Expect(stderr).To(gbytes.Say("stderr data"))

					for _, wc := range wrappedConnections {
						Eventually(wc.isClosed).Should(BeTrue())
					}
				})
			})
		})

		Context("when the process is terminated", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Expect(err).ToNot(HaveOccurred())

							Expect(payload).To(Equal(map[string]interface{}{
								"process_id": "process-handle",
								"signal":     float64(garden.SignalTerminate),
							}))

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  "process-handle",
								"exit_status": 3,
							})
						},
					),
					stdoutStream("foo-handle", "process-handle", 123, func(conn net.Conn) {
						conn.Write([]byte("stdout data"))
						conn.Write([]byte(fmt.Sprintf("roundtripped %s", <-stdInContent)))
					}),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run(context.TODO(), "foo-handle", garden.ProcessSpec{}, garden.ProcessIO{})

				Expect(err).ToNot(HaveOccurred())
				Expect(process.ID()).To(Equal("process-handle"))

				err = process.Signal(garden.SignalTerminate)
				Expect(err).ToNot(HaveOccurred())

				status, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(3))
			})
		})

		Context("when the process is killed", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Expect(err).ToNot(HaveOccurred())

							Expect(payload).To(Equal(map[string]interface{}{
								"process_id": "process-handle",
								"signal":     float64(garden.SignalKill),
							}))

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  "process-handle",
								"exit_status": 3,
							})
						},
					),
					emptyStdoutStream("foo-handle", "process-handle", 123),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run(context.TODO(), "foo-handle", garden.ProcessSpec{}, garden.ProcessIO{})

				Expect(err).ToNot(HaveOccurred())
				Expect(process.ID()).To(Equal("process-handle"))

				err = process.Signal(garden.SignalKill)
				Expect(err).ToNot(HaveOccurred())

				status, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(3))
			})
		})

		Context("when the process's window is resized", func() {
			var spec garden.ProcessSpec
			BeforeEach(func() {
				spec = garden.ProcessSpec{
					Path: "lol",
					Args: []string{"arg1", "arg2"},
					TTY: &garden.TTYSpec{
						WindowSize: &garden.WindowSize{
							Columns: 100,
							Rows:    200,
						},
					},
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.VerifyJSONRepresenting(spec),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})

							// the stdin data may come in before or after the tty message
							Eventually(func() interface{} {
								var payload map[string]interface{}
								err = decoder.Decode(&payload)
								Expect(err).ToNot(HaveOccurred())

								return payload
							}).Should(Equal(map[string]interface{}{
								"process_id": "process-handle",
								"tty": map[string]interface{}{
									"window_size": map[string]interface{}{
										"columns": float64(80),
										"rows":    float64(24),
									},
								},
							}))

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  "process-handle",
								"exit_status": 3,
							})
						},
					),
					emptyStdoutStream("foo-handle", "process-handle", 123),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run(context.TODO(), "foo-handle", spec, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: gbytes.NewBuffer(),
					Stderr: gbytes.NewBuffer(),
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(process.ID()).To(Equal("process-handle"))

				err = process.SetTTY(garden.TTYSpec{
					WindowSize: &garden.WindowSize{
						Columns: 80,
						Rows:    24,
					},
				})
				Expect(err).ToNot(HaveOccurred())

				status, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(3))
			})
		})

		Context("when the connection breaks while attaching to the streams", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, _, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})
						},
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/process-handle/attaches/123/stdout"),

						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusInternalServerError)

							conn, _, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())
							defer conn.Close()
						},
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func(done Done) {
					process, err := connection.Run(context.TODO(), "foo-handle", garden.ProcessSpec{
						Path: "lol",
						Args: []string{"arg1", "arg2"},
						Dir:  "/some/dir",
					}, garden.ProcessIO{Stdout: GinkgoWriter})

					Expect(err).ToNot(HaveOccurred())

					_, err = process.Wait()
					Expect(err).To(MatchError(ContainSubstring("connection: failed to hijack stream ")))

					close(done)
				})
			})
		})

		Context("when the connection breaks before an exit status is received", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, _, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})
						},
					),
					emptyStdoutStream("foo-handle", "process-handle", 123),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Run(context.TODO(), "foo-handle", garden.ProcessSpec{
						Path: "lol",
						Args: []string{"arg1", "arg2"},
						Dir:  "/some/dir",
					}, garden.ProcessIO{})

					Expect(err).ToNot(HaveOccurred())

					_, err = process.Wait()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the connection returns an error payload", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.RespondWith(200, marshalProto(map[string]interface{}{
							"process_id": "process-handle",
							"stream_id":  "123",
						},
							map[string]interface{}{
								"process_id": "process-handle",
								"source":     transport.Stderr,
								"data":       "stderr data",
							},
							map[string]interface{}{
								"process_id": "process-handle",
								"error":      "oh no!",
							},
						)),
					),
					emptyStdoutStream("foo-handle", "process-handle", 123),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Run(context.TODO(), "foo-handle", garden.ProcessSpec{
						Path: "lol",
						Args: []string{"arg1", "arg2"},
						Dir:  "/some/dir",
					}, garden.ProcessIO{})

					Expect(err).ToNot(HaveOccurred())

					_, err = process.Wait()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("oh no!"))
				})
			})
		})

		Context("when the connection returns an error status", func() {
			BeforeEach(func() {
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
					ghttp.RespondWith(500, "an error occurred!"),
				))
			})

			It("returns an error", func() {
				_, err := connection.Run(context.TODO(), "foo-handle", garden.ProcessSpec{
					Path: "lol",
					Args: []string{"arg1", "arg2"},
					Dir:  "/some/dir",
				}, garden.ProcessIO{})

				Expect(err).To(MatchError(ContainSubstring("an error occurred!")))
			})
		})
	})

	Describe("Attaching", func() {
		Context("when streaming succeeds to completion", func() {
			BeforeEach(func() {
				expectedRoundtrip := make(chan string)
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/process-handle"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})

							var payload map[string]interface{}
							err = json.NewDecoder(br).Decode(&payload)
							Expect(err).ToNot(HaveOccurred())

							Expect(payload).To(Equal(map[string]interface{}{
								"process_id": "process-handle",
								"source":     float64(transport.Stdin),
								"data":       "stdin data",
							}))
							expectedRoundtrip <- payload["data"].(string)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  "process-handle",
								"exit_status": 3,
							})
						},
					),
					stdoutStream("foo-handle", "process-handle", 123, func(conn net.Conn) {
						conn.Write([]byte("stdout data"))
						conn.Write([]byte(fmt.Sprintf("roundtripped %s", <-expectedRoundtrip)))
					}),
					stderrStream("foo-handle", "process-handle", 123, func(conn net.Conn) {
						conn.Write([]byte("stderr data"))
					}),
				)
			})

			It("should stream", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()

				process, err := connection.Attach(context.TODO(), "foo-handle", "process-handle", garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: stdout,
					Stderr: stderr,
				})

				Expect(err).ToNot(HaveOccurred())
				Expect(process.ID()).To(Equal("process-handle"))

				Eventually(stdout).Should(gbytes.Say("stdout data"))
				Eventually(stderr).Should(gbytes.Say("stderr data"))
				Eventually(stdout).Should(gbytes.Say("roundtripped stdin data"))

				status, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(3))
			})

			It("finishes streaming stdout and stderr before returning from .Wait", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()

				process, err := connection.Attach(context.TODO(), "foo-handle", "process-handle", garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: stdout,
					Stderr: stderr,
				})

				Expect(err).ToNot(HaveOccurred())

				process.Wait()
				Expect(stdout).To(gbytes.Say("roundtripped stdin data"))
				Expect(stderr).To(gbytes.Say("stderr data"))
			})

		})

		Context("when ctx is done", func() {

			var fakeHijacker *connectionfakes.FakeHijackStreamer
			var wrappedConnections []*wrappedConnection
			var streamCancelFunc context.CancelFunc
			var streamContext context.Context

			BeforeEach(func() {
				streamContext, streamCancelFunc = context.WithCancel(context.Background())
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/process-handle"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, _, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})
						},
					),
					stdoutStream("foo-handle", "process-handle", 123, func(conn net.Conn) {
						<-streamContext.Done()
						conn.Write([]byte("stdout data"))
					}),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)
				wrappedConnections = []*wrappedConnection{}
				netHijacker := hijacker
				fakeHijacker = new(connectionfakes.FakeHijackStreamer)
				fakeHijacker.HijackStub = func(ctx context.Context, handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (net.Conn, *bufio.Reader, error) {
					conn, resp, err := netHijacker.Hijack(ctx, handler, body, params, query, contentType)
					wc := &wrappedConnection{Conn: conn}
					wrappedConnections = append(wrappedConnections, wc)
					return wc, resp, err
				}

				hijacker = fakeHijacker
			})
			AfterEach(func() {
				streamCancelFunc()
			})

			It("should close all net.Conn from Attach and return from .Wait", func() {
				ctx, cancelFunc := context.WithCancel(context.Background())
				process, err := connection.Attach(ctx, "foo-handle", "process-handle", garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: gbytes.NewBuffer(),
					Stderr: gbytes.NewBuffer(),
				})
				Expect(err).ToNot(HaveOccurred())
				go func() {
					cancelFunc()
				}()
				process.Wait()

				for _, wc := range wrappedConnections {
					Eventually(wc.isClosed).Should(BeTrue())
				}
			})
		})

		Context("when an error occurs while reading the given stdin stream", func() {
			It("does not send an EOF to close the process's stdin", func() {
				finishedReq := make(chan struct{})

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/process-handle"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())
							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})

							decoder := json.NewDecoder(br)

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Expect(err).ToNot(HaveOccurred())

							Expect(payload).To(Equal(map[string]interface{}{
								"process_id": "process-handle",
								"source":     float64(transport.Stdin),
								"data":       "stdin data",
							}))

							close(finishedReq)
						},
					),
					emptyStdoutStream("foo-handle", "process-handle", 123),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)

				stdinR, stdinW := io.Pipe()

				_, err := connection.Attach(context.TODO(), "foo-handle", "process-handle", garden.ProcessIO{
					Stdin: stdinR,
				})
				Expect(err).ToNot(HaveOccurred())

				stdinW.Write([]byte("stdin data"))
				stdinW.CloseWithError(errors.New("connection broke"))

				Eventually(finishedReq).Should(BeClosed())
			})
		})

		Context("when the connection returns an error payload", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/process-handle"),
						ghttp.RespondWith(200, marshalProto(map[string]interface{}{
							"process_id": "process-handle",
							"stream_id":  "123",
						},
							map[string]interface{}{
								"process_id": "process-handle",
							},
							map[string]interface{}{
								"process_id": "process-handle",
								"source":     transport.Stdout,
								"data":       "stdout data",
							},
							map[string]interface{}{
								"process_id": "process-handle",
								"source":     transport.Stderr,
								"data":       "stderr data",
							},
							map[string]interface{}{
								"process_id": "process-handle",
								"error":      "oh no!",
							},
						)),
					),
					emptyStdoutStream("foo-handle", "process-handle", 123),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Attach(context.TODO(), "foo-handle", "process-handle", garden.ProcessIO{})

					Expect(err).ToNot(HaveOccurred())
					Expect(process.ID()).To(Equal("process-handle"))

					_, err = process.Wait()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("oh no!"))
				})
			})
		})

		Context("when the connection breaks before an exit status is received", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/process-handle"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, _, err := w.(http.Hijacker).Hijack()
							Expect(err).ToNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"stream_id":  "123",
							})

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"source":     transport.Stdout,
								"data":       "stdout data",
							})

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": "process-handle",
								"source":     transport.Stderr,
								"data":       "stderr data",
							})
						},
					),
					emptyStdoutStream("foo-handle", "process-handle", 123),
					emptyStderrStream("foo-handle", "process-handle", 123),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Attach(context.TODO(), "foo-handle", "process-handle", garden.ProcessIO{})

					Expect(err).ToNot(HaveOccurred())
					Expect(process.ID()).To(Equal("process-handle"))

					_, err = process.Wait()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the server returns HTTP 404", func() {
			BeforeEach(func() {
				gardenErr := garden.Error{Err: garden.ProcessNotFoundError{ProcessID: "idontexist"}}
				respBody, err := gardenErr.MarshalJSON()
				Expect(err).NotTo(HaveOccurred())
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/idontexist"),
						ghttp.RespondWith(http.StatusNotFound, respBody),
					),
				)
			})

			It("returns a ProcessNotFoundError", func() {
				_, err := connection.Attach(context.TODO(), "foo-handle", "idontexist", garden.ProcessIO{})
				Expect(err).To(MatchError(garden.ProcessNotFoundError{
					ProcessID: "idontexist",
				}))
			})
		})
	})
})

func verifyRequestBody(expectedMessage interface{}, emptyType interface{}) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		defer GinkgoRecover()

		decoder := json.NewDecoder(req.Body)

		received := emptyType
		err := decoder.Decode(&received)
		Expect(err).ToNot(HaveOccurred())

		Expect(received).To(Equal(expectedMessage))
	}
}

func marshalProto(messages ...interface{}) string {
	result := new(bytes.Buffer)
	for _, msg := range messages {
		err := transport.WriteMessage(result, msg)
		Expect(err).ToNot(HaveOccurred())
	}

	return result.String()
}

func emptyStdoutStream(handle, processid string, attachid int) http.HandlerFunc {
	return stdoutStream(handle, processid, attachid, func(net.Conn) {})
}

func emptyStderrStream(handle, processid string, attachid int) http.HandlerFunc {
	return stderrStream(handle, processid, attachid, func(net.Conn) {})
}

func stderrStream(handle, processid string, attachid int, fn func(net.Conn)) http.HandlerFunc {
	return stream(handle, "stderr", processid, attachid, fn)
}

func stdoutStream(handle, processid string, attachid int, fn func(net.Conn)) http.HandlerFunc {
	return stream(handle, "stdout", processid, attachid, fn)
}

func stream(handle, route, processid string, attachid int, fn func(net.Conn)) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyRequest("GET",
			fmt.Sprintf("/containers/%s/processes/%s/attaches/%d/%s",
				handle,
				processid,
				attachid,
				route,
			)),

		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)

			conn, _, err := w.(http.Hijacker).Hijack()
			Expect(err).ToNot(HaveOccurred())
			defer conn.Close()

			fn(conn)
		},
	)
}

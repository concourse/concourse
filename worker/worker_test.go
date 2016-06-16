package worker_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	"github.com/concourse/baggageclaim"
	bfakes "github.com/concourse/baggageclaim/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Worker", func() {
	var (
		logger                 *lagertest.TestLogger
		fakeGardenClient       *gfakes.FakeClient
		fakeBaggageclaimClient *bfakes.FakeClient
		fakeVolumeClient       *wfakes.FakeVolumeClient
		fakeVolumeFactory      *wfakes.FakeVolumeFactory
		fakeImageFetcher       *wfakes.FakeImageFetcher
		fakeGardenWorkerDB     *wfakes.FakeGardenWorkerDB
		fakeWorkerProvider     *wfakes.FakeWorkerProvider
		fakeClock              *fakeclock.FakeClock
		activeContainers       int
		resourceTypes          []atc.WorkerResourceType
		platform               string
		tags                   atc.Tags
		workerName             string
		workerStartTime        int64
		httpProxyURL           string
		httpsProxyURL          string
		noProxy                string

		gardenWorker Worker
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeGardenClient = new(gfakes.FakeClient)
		fakeBaggageclaimClient = new(bfakes.FakeClient)
		fakeVolumeClient = new(wfakes.FakeVolumeClient)
		fakeVolumeFactory = new(wfakes.FakeVolumeFactory)
		fakeImageFetcher = new(wfakes.FakeImageFetcher)
		fakeGardenWorkerDB = new(wfakes.FakeGardenWorkerDB)
		fakeWorkerProvider = new(wfakes.FakeWorkerProvider)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		activeContainers = 42
		resourceTypes = []atc.WorkerResourceType{
			{
				Type:    "some-resource",
				Image:   "some-resource-image",
				Version: "some-version",
			},
		}
		platform = "some-platform"
		tags = atc.Tags{"some", "tags"}
		workerName = "some-worker"
		workerStartTime = fakeClock.Now().Unix()

		gardenWorker = NewGardenWorker(
			fakeGardenClient,
			fakeBaggageclaimClient,
			fakeVolumeClient,
			fakeVolumeFactory,
			fakeImageFetcher,
			fakeGardenWorkerDB,
			fakeWorkerProvider,
			fakeClock,
			activeContainers,
			resourceTypes,
			platform,
			tags,
			workerName,
			workerStartTime,
			httpProxyURL,
			httpsProxyURL,
			noProxy,
		)
	})

	Describe("CreateContainer", func() {
		var (
			logger                    lager.Logger
			signals                   <-chan os.Signal
			fakeImageFetchingDelegate *wfakes.FakeImageFetchingDelegate
			containerID               Identifier
			containerMetadata         Metadata
			customTypes               atc.ResourceTypes
			containerSpec             ContainerSpec

			createdContainer Container
			createErr        error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")

			signals = make(chan os.Signal)
			fakeImageFetchingDelegate = new(wfakes.FakeImageFetchingDelegate)

			containerID = Identifier{
				BuildID: 42,
			}

			containerMetadata = Metadata{
				BuildName: "lol",
			}

			customTypes = atc.ResourceTypes{
				{
					Name:   "custom-type-b",
					Type:   "custom-type-a",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-a",
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-c",
					Type:   "custom-type-b",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-d",
					Type:   "custom-type-b",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "unknown-custom-type",
					Type:   "unknown-base-type",
					Source: atc.Source{"some": "source"},
				},
			}
		})

		JustBeforeEach(func() {
			createdContainer, createErr = gardenWorker.CreateContainer(logger, signals, fakeImageFetchingDelegate, containerID, containerMetadata, containerSpec, customTypes)
		})

		BeforeEach(func() {
			containerSpec = ContainerSpec{
				ImageSpec: ImageSpec{
					ImageURL:   "some-image",
					Privileged: true,
				},
			}

			fakeContainer := new(gfakes.FakeContainer)
			fakeContainer.HandleReturns("some-container-handle")

			fakeGardenClient.CreateReturns(fakeContainer, nil)
		})

		It("tries to create a container in garden", func() {
			Expect(createErr).NotTo(HaveOccurred())
			Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
			actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
			expectedEnv := containerSpec.Env
			Expect(actualGardenSpec.Env).To(Equal(expectedEnv))
			Expect(actualGardenSpec.Properties["user"]).To(Equal(""))
			Expect(actualGardenSpec.Privileged).To(BeTrue())
			Expect(actualGardenSpec.RootFSPath).To(Equal("some-image"))
		})

		It("tries to create the container in the db", func() {
			Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
			c, ttl, maxContainerLifetime := fakeGardenWorkerDB.CreateContainerArgsForCall(0)

			Expect(c).To(Equal(db.Container{
				ContainerIdentifier: db.ContainerIdentifier(Identifier{
					BuildID: 42,
				}),
				ContainerMetadata: db.ContainerMetadata(Metadata{
					BuildName:  "lol",
					Handle:     "some-container-handle",
					WorkerName: "some-worker",
				}),
			}))

			Expect(ttl).To(Equal(ContainerTTL))
			Expect(maxContainerLifetime).To(Equal(time.Duration(0)))
		})

		Context("when the spec does not specify ImageURL", func() {
			BeforeEach(func() {
				containerSpec.ImageSpec.ImageURL = ""
			})

			It("tries to create a container in garden with an empty RootFSPath", func() {
				Expect(createErr).NotTo(HaveOccurred())
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.RootFSPath).To(BeEmpty())
			})
		})

		Context("when creating the container succeeds", func() {
			var fakeContainer *gfakes.FakeContainer
			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-container-handle")
				fakeGardenClient.CreateReturns(fakeContainer, nil)
				fakeGardenWorkerDB.GetContainerReturns(db.SavedContainer{
					TTL: 5 * time.Minute,
				}, true, nil)
			})

			It("returns a container that be destroyed", func() {
				err := createdContainer.Destroy()
				Expect(err).NotTo(HaveOccurred())

				By("destroying via garden")
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-container-handle"))

				By("no longer heartbeating")
				fakeClock.Increment(30 * time.Second)
				Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
			})

			It("performs an initial heartbeat synchronously on the returned container", func() {
				Expect(fakeContainer.SetGraceTimeCallCount()).To(Equal(1))
				Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).To(Equal(1))
			})

			It("heartbeats to the database and the container", func() {
				fakeClock.Increment(30 * time.Second)

				Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
				Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(5 * time.Minute))

				Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
				handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
				Expect(handle).To(Equal("some-container-handle"))
				Expect(interval).To(Equal(5 * time.Minute))

				fakeClock.Increment(30 * time.Second)

				Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(3))
				Expect(fakeContainer.SetGraceTimeArgsForCall(2)).To(Equal(5 * time.Minute))

				Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(3))
				handle, interval = fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(2)
				Expect(handle).To(Equal("some-container-handle"))
				Expect(interval).To(Equal(5 * time.Minute))
			})

			It("sets a final ttl on the container and stops heartbeating when the container is released", func() {
				createdContainer.Release(FinalTTL(30 * time.Minute))

				Expect(fakeContainer.SetGraceTimeCallCount()).Should(Equal(2))
				Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(30 * time.Minute))

				Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).Should(Equal(2))
				handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
				Expect(handle).To(Equal("some-container-handle"))
				Expect(interval).To(Equal(30 * time.Minute))

				fakeClock.Increment(30 * time.Second)

				Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
				Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
			})

			It("does not perform a final heartbeat when there is no final ttl", func() {
				createdContainer.Release(nil)

				Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
				Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(1))
			})

			Context("when creating the container in the db fails", func() {
				var gardenWorkerDBCreateContainerErr error
				BeforeEach(func() {
					gardenWorkerDBCreateContainerErr = errors.New("an-error")
					fakeGardenWorkerDB.CreateContainerReturns(db.SavedContainer{}, gardenWorkerDBCreateContainerErr)
				})

				It("returns the error", func() {
					Expect(createErr).To(Equal(gardenWorkerDBCreateContainerErr))
				})
			})

			Context("when creating the container in the db succeeds", func() {
				BeforeEach(func() {
					fakeGardenWorkerDB.CreateContainerReturns(db.SavedContainer{}, nil)
				})

				It("returns a Container", func() {
					Expect(createdContainer).NotTo(BeNil())
				})
			})
		})

		Context("when creating the container fails", func() {
			var gardenCreateErr error

			BeforeEach(func() {
				gardenCreateErr = errors.New("an-error")
				fakeGardenClient.CreateReturns(nil, gardenCreateErr)
			})

			It("returns the error", func() {
				Expect(createErr).To(HaveOccurred())
				Expect(createErr).To(Equal(gardenCreateErr))
			})
		})

		Context("when the spec specifies Inputs", func() {
			var (
				volume1    *wfakes.FakeVolume
				volume2    *wfakes.FakeVolume
				cowVolume1 *wfakes.FakeVolume
				cowVolume2 *wfakes.FakeVolume
			)

			BeforeEach(func() {
				volume1 = new(wfakes.FakeVolume)
				volume1.HandleReturns("vol-1-handle")
				volume2 = new(wfakes.FakeVolume)
				volume2.HandleReturns("vol-2-handle")

				containerSpec.Inputs = []VolumeMount{
					{
						volume1,
						"vol-1-mount-path",
					},
					{
						volume2,
						"vol-2-mount-path",
					},
				}

				cowVolume1 = new(wfakes.FakeVolume)
				cowVolume1.HandleReturns("cow-vol-1-handle")
				cowVolume2 = new(wfakes.FakeVolume)
				cowVolume2.HandleReturns("cow-vol-2-handle")

				fakeVolumeClient.CreateVolumeStub = func(logger lager.Logger, volumeSpec VolumeSpec) (Volume, error) {
					s, ok := volumeSpec.Strategy.(ContainerRootFSStrategy)
					Expect(ok).To(BeTrue())

					switch s.Parent.Handle() {
					case "vol-1-handle":
						return cowVolume1, nil
					case "vol-2-handle":
						return cowVolume2, nil
					default:
						panic("unexpected handle: " + s.Parent.Handle())
					}
				}
			})

			It("creates a COW volume for each mount", func() {
				Expect(fakeVolumeClient.CreateVolumeCallCount()).To(Equal(2))
				_, volumeSpec := fakeVolumeClient.CreateVolumeArgsForCall(0)
				Expect(volumeSpec).To(Equal(VolumeSpec{
					Strategy: ContainerRootFSStrategy{
						Parent: volume1,
					},
					Privileged: true,
					TTL:        VolumeTTL,
				}))

				_, volumeSpec = fakeVolumeClient.CreateVolumeArgsForCall(1)
				Expect(volumeSpec).To(Equal(VolumeSpec{
					Strategy: ContainerRootFSStrategy{
						Parent: volume2,
					},
					Privileged: true,
					TTL:        VolumeTTL,
				}))
			})

			Context("when creating any volume fails", func() {
				var disaster error
				BeforeEach(func() {
					disaster = errors.New("an-error")
					fakeVolumeClient.CreateVolumeStub = func(logger lager.Logger, volumeSpec VolumeSpec) (Volume, error) {
						s := volumeSpec.Strategy.(ContainerRootFSStrategy)
						switch s.Parent.Handle() {
						case "vol-1-handle":
							return cowVolume1, nil
						case "vol-2-handle":
							return nil, disaster
						}
						return new(wfakes.FakeVolume), nil
					}
				})

				It("returns the error", func() {
					Expect(createErr).To(Equal(disaster))
				})
			})

			It("releases each cow volume after attempting to create the container", func() {
				Expect(cowVolume1.ReleaseCallCount()).To(Equal(1))
				Expect(cowVolume1.ReleaseArgsForCall(0)).To(BeNil())
				Expect(cowVolume2.ReleaseCallCount()).To(Equal(1))
				Expect(cowVolume2.ReleaseArgsForCall(0)).To(BeNil())
			})

			It("does not release the volumes that were passed in", func() {
				Expect(volume1.ReleaseCallCount()).To(BeZero())
				Expect(volume2.ReleaseCallCount()).To(BeZero())
			})

			It("adds each cow volume to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				concourseVolumes := []string{}
				err := json.Unmarshal([]byte(actualGardenSpec.Properties["concourse:volumes"]), &concourseVolumes)
				Expect(err).NotTo(HaveOccurred())
				Expect(concourseVolumes).To(ContainElement("cow-vol-1-handle"))
				Expect(concourseVolumes).To(ContainElement("cow-vol-2-handle"))
			})

			It("adds each cow volume to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				volumeMountProperties := map[string]string{}
				err := json.Unmarshal([]byte(actualGardenSpec.Properties["concourse:volume-mounts"]), &volumeMountProperties)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumeMountProperties["cow-vol-1-handle"]).To(Equal("vol-1-mount-path"))
				Expect(volumeMountProperties["cow-vol-2-handle"]).To(Equal("vol-2-mount-path"))
			})
		})

		Context("when the spec specifies Outputs", func() {
			var (
				volume1 *wfakes.FakeVolume
				volume2 *wfakes.FakeVolume
			)

			BeforeEach(func() {
				volume1 = new(wfakes.FakeVolume)
				volume1.HandleReturns("vol-1-handle")
				volume1.PathReturns("vol-1-path")
				volume2 = new(wfakes.FakeVolume)
				volume2.HandleReturns("vol-2-handle")
				volume2.PathReturns("vol-2-path")

				containerSpec.Outputs = []VolumeMount{
					{
						volume1,
						"vol-1-mount-path",
					},
					{
						volume2,
						"vol-2-mount-path",
					},
				}
			})

			It("creates a bind mount for each output volume", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.BindMounts).To(ConsistOf([]garden.BindMount{
					{
						SrcPath: "vol-1-path",
						DstPath: "vol-1-mount-path",
						Mode:    garden.BindMountModeRW,
					},
					{
						SrcPath: "vol-2-path",
						DstPath: "vol-2-mount-path",
						Mode:    garden.BindMountModeRW,
					},
				}))
			})

			It("adds each output volume to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				concourseVolumes := []string{}
				err := json.Unmarshal([]byte(actualGardenSpec.Properties["concourse:volumes"]), &concourseVolumes)
				Expect(err).NotTo(HaveOccurred())
				Expect(concourseVolumes).To(ConsistOf([]string{"vol-1-handle", "vol-2-handle"}))
			})
		})

		Context("when the spec specifies ImageResource", func() {
			var (
				imageVolume  *wfakes.FakeVolume
				imageVersion atc.Version
			)

			BeforeEach(func() {
				containerMetadata.Type = db.ContainerTypeTask
				containerSpec.ImageSpec.ImageResource = &atc.ImageResource{
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				}

				imageVolume = new(wfakes.FakeVolume)
				imageVolume.HandleReturns("image-volume")
				imageVolume.PathReturns("/some/image/path")

				metadataReader := ioutil.NopCloser(strings.NewReader(
					`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
				))

				imageVersion = atc.Version{"image": "version"}

				fakeImageFetcher.FetchImageReturns(imageVolume, metadataReader, imageVersion, nil)
			})

			It("tries to create the container in the db", func() {
				Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
				c, ttl, maxContainerLifetime := fakeGardenWorkerDB.CreateContainerArgsForCall(0)

				expectedContainerID := Identifier{
					BuildID:             42,
					ResourceTypeVersion: atc.Version{"image": "version"},
				}

				expectedContainerMetadata := Metadata{
					BuildName:  "lol",
					Handle:     "some-container-handle",
					User:       "image-volume-user",
					WorkerName: "some-worker",
					Type:       "task",
				}

				Expect(c).To(Equal(db.Container{
					ContainerIdentifier: db.ContainerIdentifier(expectedContainerID),
					ContainerMetadata:   db.ContainerMetadata(expectedContainerMetadata),
				}))

				Expect(ttl).To(Equal(ContainerTTL))
				Expect(maxContainerLifetime).To(Equal(time.Duration(0)))
			})

			It("tries to fetch the image for the resource type", func() {
				Expect(fakeImageFetcher.FetchImageCallCount()).To(Equal(1))
				_, fetchImageConfig, fetchSignals, fetchID, fetchMetadata, fetchDelegate, fetchWorker, fetchTags, fetchCustomTypes, fetchPrivileged := fakeImageFetcher.FetchImageArgsForCall(0)
				Expect(fetchImageConfig).To(Equal(atc.ImageResource{
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				}))
				Expect(fetchSignals).To(Equal(signals))
				Expect(fetchID).To(Equal(containerID))
				Expect(fetchMetadata).To(Equal(containerMetadata))
				Expect(fetchDelegate).To(Equal(fakeImageFetchingDelegate))
				Expect(fetchWorker).To(Equal(gardenWorker))
				Expect(fetchTags).To(Equal(atc.Tags{"some", "tags"}))
				Expect(fetchCustomTypes).To(Equal(customTypes))
				Expect(fetchPrivileged).To(Equal(true))
			})

			It("creates the container with the fetched image's URL as the rootfs", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.RootFSPath).To(Equal("raw:///some/image/path/rootfs"))
			})

			It("adds the image env to the garden spec", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				expectedEnv := append([]string{"A=1", "B=2"}, containerSpec.Env...)
				Expect(actualGardenSpec.Env).To(Equal(expectedEnv))
			})

			It("adds the image volume to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				concourseVolumes := []string{}
				err := json.Unmarshal([]byte(actualGardenSpec.Properties["concourse:volumes"]), &concourseVolumes)
				Expect(err).NotTo(HaveOccurred())
				Expect(concourseVolumes).To(ContainElement("image-volume"))
			})

			It("adds the image user to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.Properties["user"]).To(Equal("image-volume-user"))
			})

			Context("when fetching the image fails", func() {
				BeforeEach(func() {
					fakeImageFetcher.FetchImageReturns(nil, nil, nil, errors.New("fetch-err"))
				})

				It("returns an error", func() {
					Expect(createErr).To(HaveOccurred())
					Expect(createErr.Error()).To(Equal("fetch-err"))
				})
			})

			It("releases the cow volume after attempting to create the container", func() {
				Expect(imageVolume.ReleaseCallCount()).To(Equal(1))
				Expect(imageVolume.ReleaseArgsForCall(0)).To(BeNil())
			})

			Context("when the metadata.json is bogus", func() {
				BeforeEach(func() {
					fakeImageFetcher.FetchImageReturns(imageVolume, ioutil.NopCloser(strings.NewReader(`{"env": 42}`)), imageVersion, nil)
				})

				It("returns ErrMalformedMetadata", func() {
					Expect(createErr).To(BeAssignableToTypeOf(MalformedMetadataError{}))
					Expect(createErr.Error()).To(Equal(fmt.Sprintf("malformed image metadata: json: cannot unmarshal number into Go value of type []string")))
				})
			})
		})

		Context("when the spec specifies ResourceType", func() {
			var (
				imageVolume  *wfakes.FakeVolume
				imageVersion atc.Version
			)

			BeforeEach(func() {
				containerMetadata.Type = db.ContainerTypeGet
				containerSpec = ContainerSpec{
					ImageSpec: ImageSpec{
						ResourceType: "custom-type-a",
						Privileged:   true,
					},
					Env: []string{"env-1", "env-2"},
				}

				imageVolume = new(wfakes.FakeVolume)
				imageVolume.HandleReturns("image-volume")
				imageVolume.PathReturns("/some/image/path")

				metadataReader := ioutil.NopCloser(strings.NewReader(
					`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
				))

				imageVersion := atc.Version{"image": "version"}

				fakeImageFetcher.FetchImageReturns(imageVolume, metadataReader, imageVersion, nil)
			})

			It("tries to create the container in the db", func() {
				Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
				c, ttl, maxContainerLifetime := fakeGardenWorkerDB.CreateContainerArgsForCall(0)

				expectedContainerID := Identifier{
					BuildID:             42,
					ResourceTypeVersion: atc.Version{"image": "version"},
				}

				expectedContainerMetadata := Metadata{
					BuildName:  "lol",
					Handle:     "some-container-handle",
					User:       "image-volume-user",
					WorkerName: "some-worker",
					Type:       "get",
				}

				Expect(c).To(Equal(db.Container{
					ContainerIdentifier: db.ContainerIdentifier(expectedContainerID),
					ContainerMetadata:   db.ContainerMetadata(expectedContainerMetadata),
				}))

				Expect(ttl).To(Equal(ContainerTTL))
				Expect(maxContainerLifetime).To(Equal(time.Duration(0)))
			})

			It("tries to fetch the image for the resource type", func() {
				Expect(fakeImageFetcher.FetchImageCallCount()).To(Equal(1))
				_, fetchImageConfig, fetchSignals, fetchID, fetchMetadata, fetchDelegate, fetchWorker, fetchTags, fetchCustomTypes, fetchPrivileged := fakeImageFetcher.FetchImageArgsForCall(0)
				Expect(fetchImageConfig).To(Equal(atc.ImageResource{
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				}))
				Expect(fetchSignals).To(Equal(signals))
				Expect(fetchID).To(Equal(containerID))
				Expect(fetchMetadata).To(Equal(containerMetadata))
				Expect(fetchDelegate).To(Equal(fakeImageFetchingDelegate))
				Expect(fetchWorker).To(Equal(gardenWorker))
				Expect(fetchTags).To(Equal(atc.Tags{"some", "tags"}))
				Expect(fetchCustomTypes).To(Equal(customTypes.Without("custom-type-a")))
				Expect(fetchPrivileged).To(Equal(true))
			})

			It("creates the container with the fetched image's URL as the rootfs", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.RootFSPath).To(Equal("raw:///some/image/path/rootfs"))
			})

			It("adds the image env to the garden spec", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				expectedEnv := append([]string{"A=1", "B=2"}, containerSpec.Env...)
				Expect(actualGardenSpec.Env).To(Equal(expectedEnv))
			})

			It("adds the image volume to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				concourseVolumes := []string{}
				err := json.Unmarshal([]byte(actualGardenSpec.Properties["concourse:volumes"]), &concourseVolumes)
				Expect(err).NotTo(HaveOccurred())
				Expect(concourseVolumes).To(ContainElement("image-volume"))
			})

			It("adds the image user to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.Properties["user"]).To(Equal("image-volume-user"))
			})

			Context("when fetching the image fails", func() {
				BeforeEach(func() {
					fakeImageFetcher.FetchImageReturns(nil, nil, nil, errors.New("fetch-err"))
				})

				It("returns an error", func() {
					Expect(createErr).To(HaveOccurred())
					Expect(createErr.Error()).To(Equal("fetch-err"))
				})
			})

			It("sets Privileged to true in the garden spec", func() {
				Expect(createErr).NotTo(HaveOccurred())
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.Privileged).To(BeTrue())
			})

			Context("when the spec specifies Ephemeral", func() {
				BeforeEach(func() {
					containerSpec.Ephemeral = true
				})

				It("adds concourse:ephemeral = true to the garden spec properties", func() {
					Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
					actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
					Expect(actualGardenSpec.Properties["concourse:ephemeral"]).To(Equal("true"))
				})
			})

			It("releases the cow volume after attempting to create the container", func() {
				Expect(imageVolume.ReleaseCallCount()).To(Equal(1))
				Expect(imageVolume.ReleaseArgsForCall(0)).To(BeNil())
			})

			Context("when the metadata.json is bogus", func() {
				BeforeEach(func() {
					fakeImageFetcher.FetchImageReturns(imageVolume, ioutil.NopCloser(strings.NewReader(`{"env": 42}`)), imageVersion, nil)
				})

				It("returns ErrMalformedMetadata", func() {
					Expect(createErr).To(BeAssignableToTypeOf(MalformedMetadataError{}))
					Expect(createErr.Error()).To(Equal(fmt.Sprintf("malformed image metadata: json: cannot unmarshal number into Go value of type []string")))
				})
			})

			Context("when the resource type is one that a worker provides", func() {
				var importVolume *wfakes.FakeVolume
				var cowVolume *wfakes.FakeVolume

				BeforeEach(func() {
					containerSpec.ImageSpec.ResourceType = "some-resource"

					importVolume = new(wfakes.FakeVolume)
					importVolume.HandleReturns("import-vol")

					cowVolume = new(wfakes.FakeVolume)
					cowVolume.HandleReturns("cow-vol")
					cowVolume.PathReturns("cow-vol-path")

					fakeVolumeClient.CreateVolumeStub = func(logger lager.Logger, volumeSpec VolumeSpec) (Volume, error) {
						switch volumeSpec.Strategy.(type) {
						case HostRootFSStrategy:
							return importVolume, nil
						case ContainerRootFSStrategy:
							return cowVolume, nil
						default:
							return new(wfakes.FakeVolume), nil
						}
					}

					fakeVolumeClient.FindVolumeReturns(importVolume, true, nil)
				})

				It("tries to find an existing import volume", func() {
					Expect(fakeVolumeClient.FindVolumeCallCount()).To(Equal(1))
					_, actualVolumeSpec := fakeVolumeClient.FindVolumeArgsForCall(0)
					version := "some-version"
					Expect(actualVolumeSpec).To(Equal(VolumeSpec{
						Strategy: HostRootFSStrategy{
							Path:       "some-resource-image",
							Version:    &version,
							WorkerName: "some-worker",
						},
						Privileged: true,
						Properties: VolumeProperties{},
						TTL:        0,
					}))
				})

				It("tries to create a COW volume with the import volume as its parent", func() {
					Expect(fakeVolumeClient.CreateVolumeCallCount()).To(Equal(1))
					_, actualVolumeSpec := fakeVolumeClient.CreateVolumeArgsForCall(0)
					Expect(actualVolumeSpec).To(Equal(VolumeSpec{
						Strategy: ContainerRootFSStrategy{
							Parent: importVolume,
						},
						Privileged: true,
						Properties: VolumeProperties{},
						TTL:        5 * time.Minute,
					}))
				})

				Context("when the import volume cannot be retrieved", func() {
					BeforeEach(func() {
						fakeVolumeClient.FindVolumeReturns(nil, false, nil)
					})

					It("creates import and COW volumes for the resource image", func() {
						Expect(fakeVolumeClient.CreateVolumeCallCount()).To(Equal(2))
						_, actualVolumeSpec := fakeVolumeClient.CreateVolumeArgsForCall(0)
						version := "some-version"
						Expect(actualVolumeSpec).To(Equal(VolumeSpec{
							Strategy: HostRootFSStrategy{
								Path:       "some-resource-image",
								Version:    &version,
								WorkerName: "some-worker",
							},
							Privileged: true,
							Properties: VolumeProperties{},
							TTL:        0,
						}))

						_, actualVolumeSpec = fakeVolumeClient.CreateVolumeArgsForCall(1)
						Expect(actualVolumeSpec).To(Equal(VolumeSpec{
							Strategy: ContainerRootFSStrategy{
								Parent: importVolume,
							},
							Privileged: true,
							Properties: VolumeProperties{},
							TTL:        5 * time.Minute,
						}))
					})

					Context("when creating the import volume fails", func() {
						var disaster error
						BeforeEach(func() {
							disaster = errors.New("failed-to-create-volume")

							fakeVolumeClient.CreateVolumeStub = func(logger lager.Logger, volumeSpec VolumeSpec) (Volume, error) {
								switch volumeSpec.Strategy.(type) {
								case HostRootFSStrategy:
									return nil, disaster
								case ContainerRootFSStrategy:
									return cowVolume, nil
								default:
									return nil, nil
								}
							}
						})

						It("returns the error", func() {
							Expect(createErr).To(Equal(disaster))
						})
					})

					Context("when creating the COW volume fails", func() {
						var disaster error
						BeforeEach(func() {
							disaster = errors.New("failed-to-create-volume")

							fakeVolumeClient.CreateVolumeStub = func(logger lager.Logger, volumeSpec VolumeSpec) (Volume, error) {
								switch volumeSpec.Strategy.(type) {
								case HostRootFSStrategy:
									return importVolume, nil
								case ContainerRootFSStrategy:
									return nil, disaster
								default:
									return nil, nil
								}
							}
						})

						It("returns the error", func() {
							Expect(createErr).To(Equal(disaster))
						})
					})
				})

				It("releases the import volume", func() {
					Expect(importVolume.ReleaseCallCount()).To(Equal(1))
					Expect(importVolume.ReleaseArgsForCall(0)).To(BeNil())
				})

				It("releases the cow volume after attempting to create the container", func() {
					Expect(cowVolume.ReleaseCallCount()).To(Equal(1))
					Expect(cowVolume.ReleaseArgsForCall(0)).To(BeNil())
				})

				It("adds the cow volume to the garden spec properties", func() {
					Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
					actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
					concourseVolumes := []string{}
					err := json.Unmarshal([]byte(actualGardenSpec.Properties["concourse:volumes"]), &concourseVolumes)
					Expect(err).NotTo(HaveOccurred())
					Expect(concourseVolumes).To(ContainElement("cow-vol"))
				})

				It("does not add the cow volume mount to the garden spec properties", func() {
					Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
					actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
					volumeMountProperties := map[string]string{}
					err := json.Unmarshal([]byte(actualGardenSpec.Properties["concourse:volume-mounts"]), &volumeMountProperties)
					Expect(err).NotTo(HaveOccurred())
					Expect(volumeMountProperties).NotTo(HaveKey("cow-vol"))
				})

				It("uses the path of the cow volume as the rootfs", func() {
					Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
					actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
					Expect(actualGardenSpec.RootFSPath).To(Equal("raw://cow-vol-path"))
				})
			})

			Context("when the spec specifies a resource type that is unknown", func() {
				BeforeEach(func() {
					containerSpec.ImageSpec.ResourceType = "some-bogus-resource"
				})

				It("returns ErrUnsupportedResourceType", func() {
					Expect(createErr).To(Equal(ErrUnsupportedResourceType))
				})
			})
		})

		Context("when the spec specifies ImageVolumeAndMetadata", func() {
			var imageVolume *wfakes.FakeVolume

			BeforeEach(func() {
				imageVolume = new(wfakes.FakeVolume)
				imageVolume.HandleReturns("image-volume")
				imageVolume.PathReturns("/some/image/path")

				metadataReader := ioutil.NopCloser(strings.NewReader(
					`{"env": ["A=1", "B=2"], "user":"image-volume-user"}`,
				))

				containerMetadata.Type = db.ContainerTypeTask
				containerSpec.ImageSpec.ImageVolumeAndMetadata = ImageVolumeAndMetadata{
					Volume:         imageVolume,
					MetadataReader: metadataReader,
				}
			})

			It("tries to create the container in the db", func() {
				Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
				c, ttl, maxContainerLifetime := fakeGardenWorkerDB.CreateContainerArgsForCall(0)

				expectedContainerID := Identifier{
					BuildID: 42,
				}

				expectedContainerMetadata := Metadata{
					BuildName:  "lol",
					Handle:     "some-container-handle",
					User:       "image-volume-user",
					WorkerName: "some-worker",
					Type:       "task",
				}

				Expect(c).To(Equal(db.Container{
					ContainerIdentifier: db.ContainerIdentifier(expectedContainerID),
					ContainerMetadata:   db.ContainerMetadata(expectedContainerMetadata),
				}))

				Expect(ttl).To(Equal(ContainerTTL))
				Expect(maxContainerLifetime).To(Equal(time.Duration(0)))
			})

			It("creates the container with the fetched image's URL as the rootfs", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.RootFSPath).To(Equal("raw:///some/image/path/rootfs"))
			})

			It("adds the image env to the garden spec", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				expectedEnv := append([]string{"A=1", "B=2"}, containerSpec.Env...)
				Expect(actualGardenSpec.Env).To(Equal(expectedEnv))
			})

			It("adds the image volume to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				concourseVolumes := []string{}
				err := json.Unmarshal([]byte(actualGardenSpec.Properties["concourse:volumes"]), &concourseVolumes)
				Expect(err).NotTo(HaveOccurred())
				Expect(concourseVolumes).To(ContainElement("image-volume"))
			})

			It("adds the image user to the garden spec properties", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.Properties["user"]).To(Equal("image-volume-user"))
			})

			It("releases the cow volume after attempting to create the container", func() {
				Expect(imageVolume.ReleaseCallCount()).To(Equal(1))
				Expect(imageVolume.ReleaseArgsForCall(0)).To(BeNil())
			})

			Context("when the metadata.json is bogus", func() {
				BeforeEach(func() {
					containerSpec.ImageSpec.ImageVolumeAndMetadata.MetadataReader = ioutil.NopCloser(strings.NewReader(`{"env": 42}`))
				})

				It("returns ErrMalformedMetadata", func() {
					Expect(createErr).To(BeAssignableToTypeOf(MalformedMetadataError{}))
					Expect(createErr.Error()).To(Equal(fmt.Sprintf("malformed image metadata: json: cannot unmarshal number into Go value of type []string")))
				})
			})
		})

		Context("when the spec is for a check container", func() {
			BeforeEach(func() {
				containerMetadata.Type = db.ContainerTypeCheck
			})

			Context("when the worker has been up for less than 5 minutes", func() {
				BeforeEach(func() {
					fakeClock.IncrementBySeconds(299)
				})

				It("creates the container with a max lifetime of 5 minutes", func() {
					Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
					_, _, maxContainerLifetime := fakeGardenWorkerDB.CreateContainerArgsForCall(0)
					Expect(maxContainerLifetime).To(Equal(5 * time.Minute))
				})
			})

			Context("when the worker has been up for greater than 5 minutes, and less than an hour", func() {
				var origUptime time.Duration
				BeforeEach(func() {
					origUptime = gardenWorker.Uptime()
					fakeClock.IncrementBySeconds(301)
				})

				It("creates the container with a max lifetime equivalent to the worker uptime", func() {
					Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
					_, _, maxContainerLifetime := fakeGardenWorkerDB.CreateContainerArgsForCall(0)
					Expect(maxContainerLifetime).To(Equal(origUptime + (301 * time.Second)))
				})
			})

			Context("when the worker has been up for greater than an hour", func() {
				BeforeEach(func() {
					fakeClock.IncrementBySeconds(3601)
				})

				It("creates the container with a max lifetime of 1 hour", func() {
					Expect(fakeGardenWorkerDB.CreateContainerCallCount()).To(Equal(1))
					_, _, maxContainerLifetime := fakeGardenWorkerDB.CreateContainerArgsForCall(0)
					Expect(maxContainerLifetime).To(Equal(1 * time.Hour))
				})
			})
		})

		Context("when the worker has a HTTPProxyURL", func() {
			BeforeEach(func() {
				gardenWorker = NewGardenWorker(
					fakeGardenClient,
					fakeBaggageclaimClient,
					fakeVolumeClient,
					fakeVolumeFactory,
					fakeImageFetcher,
					fakeGardenWorkerDB,
					fakeWorkerProvider,
					fakeClock,
					activeContainers,
					resourceTypes,
					platform,
					tags,
					workerName,
					workerStartTime,
					"http://example.com",
					httpsProxyURL,
					noProxy,
				)
			})

			It("adds the proxy url to the garden spec env", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.Env).To(ContainElement("http_proxy=http://example.com"))
			})
		})

		Context("when the worker has NoProxy", func() {
			BeforeEach(func() {
				gardenWorker = NewGardenWorker(
					fakeGardenClient,
					fakeBaggageclaimClient,
					fakeVolumeClient,
					fakeVolumeFactory,
					fakeImageFetcher,
					fakeGardenWorkerDB,
					fakeWorkerProvider,
					fakeClock,
					activeContainers,
					resourceTypes,
					platform,
					tags,
					workerName,
					workerStartTime,
					httpProxyURL,
					httpsProxyURL,
					"localhost",
				)
			})

			It("adds the proxy url to the garden spec env", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.Env).To(ContainElement("no_proxy=localhost"))
			})
		})

		Context("when the worker has a HTTPSProxyURL", func() {
			BeforeEach(func() {
				gardenWorker = NewGardenWorker(
					fakeGardenClient,
					fakeBaggageclaimClient,
					fakeVolumeClient,
					fakeVolumeFactory,
					fakeImageFetcher,
					fakeGardenWorkerDB,
					fakeWorkerProvider,
					fakeClock,
					activeContainers,
					resourceTypes,
					platform,
					tags,
					workerName,
					workerStartTime,
					httpProxyURL,
					"https://example.com",
					noProxy,
				)
			})

			It("adds the proxy url to the garden spec env", func() {
				Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				actualGardenSpec := fakeGardenClient.CreateArgsForCall(0)
				Expect(actualGardenSpec.Env).To(ContainElement("https_proxy=https://example.com"))
			})
		})
	})

	Describe("LookupContainer", func() {
		var handle string

		BeforeEach(func() {
			handle = "we98lsv"
		})

		Context("when the gardenClient returns a container and no error", func() {
			var (
				fakeContainer *gfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")
				fakeGardenWorkerDB.GetContainerReturns(db.SavedContainer{}, true, nil)
				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			It("returns the container and no error", func() {
				foundContainer, found, err := gardenWorker.LookupContainer(logger, handle)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(foundContainer.Handle()).To(Equal(fakeContainer.Handle()))
			})

			Describe("the container", func() {
				var foundContainer Container
				var findErr error

				JustBeforeEach(func() {
					foundContainer, _, findErr = gardenWorker.LookupContainer(logger, handle)
				})

				Context("when the concourse:volumes property is present", func() {
					var (
						handle1Volume         *wfakes.FakeVolume
						handle2Volume         *wfakes.FakeVolume
						expectedHandle1Volume *wfakes.FakeVolume
						expectedHandle2Volume *wfakes.FakeVolume
					)

					BeforeEach(func() {
						handle1Volume = new(wfakes.FakeVolume)
						handle2Volume = new(wfakes.FakeVolume)
						expectedHandle1Volume = new(wfakes.FakeVolume)
						expectedHandle2Volume = new(wfakes.FakeVolume)

						fakeContainer.PropertiesReturns(garden.Properties{
							"concourse:volumes":       `["handle-1","handle-2"]`,
							"concourse:volume-mounts": `{"handle-1":"/handle-1/path","handle-2":"/handle-2/path"}`,
						}, nil)

						fakeBaggageclaimClient.LookupVolumeStub = func(logger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {
							if handle == "handle-1" {
								return handle1Volume, true, nil
							} else if handle == "handle-2" {
								return handle2Volume, true, nil
							} else {
								panic("unknown handle: " + handle)
							}
						}

						fakeVolumeFactory.BuildStub = func(logger lager.Logger, vol baggageclaim.Volume) (Volume, bool, error) {
							if vol == handle1Volume {
								return expectedHandle1Volume, true, nil
							} else if vol == handle2Volume {
								return expectedHandle2Volume, true, nil
							} else {
								panic("unknown volume: " + vol.Handle())
							}
						}
					})

					Describe("Volumes", func() {
						It("returns all bound volumes based on properties on the container", func() {
							Expect(foundContainer.Volumes()).To(Equal([]Volume{
								expectedHandle1Volume,
								expectedHandle2Volume,
							}))
						})

						Context("when LookupVolume returns an error", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeBaggageclaimClient.LookupVolumeReturns(nil, false, disaster)
							})

							It("returns the error on lookup", func() {
								Expect(findErr).To(Equal(disaster))
							})
						})

						Context("when LookupVolume cannot find the volume", func() {
							BeforeEach(func() {
								fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
							})

							It("returns ErrMissingVolume", func() {
								Expect(findErr).To(Equal(ErrMissingVolume))
							})
						})

						Context("when Build cannot find the volume", func() {
							BeforeEach(func() {
								fakeVolumeFactory.BuildReturns(nil, false, nil)
							})

							It("returns ErrMissingVolume", func() {
								Expect(findErr).To(Equal(ErrMissingVolume))
							})
						})

						Context("when Build returns an error", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeVolumeFactory.BuildReturns(nil, false, disaster)
							})

							It("returns the error on lookup", func() {
								Expect(findErr).To(Equal(disaster))
							})
						})

						Context("when there is no baggageclaim", func() {
							BeforeEach(func() {
								gardenWorker = NewGardenWorker(
									fakeGardenClient,
									nil,
									fakeVolumeClient,
									nil,
									fakeImageFetcher,
									fakeGardenWorkerDB,
									fakeWorkerProvider,
									fakeClock,
									activeContainers,
									resourceTypes,
									platform,
									tags,
									workerName,
									workerStartTime,
									httpProxyURL,
									httpsProxyURL,
									noProxy,
								)
							})

							It("returns an empty slice", func() {
								Expect(foundContainer.Volumes()).To(BeEmpty())
							})
						})
					})

					Describe("VolumeMounts", func() {
						It("returns all bound volumes based on properties on the container", func() {
							Expect(foundContainer.VolumeMounts()).To(ConsistOf([]VolumeMount{
								{Volume: expectedHandle1Volume, MountPath: "/handle-1/path"},
								{Volume: expectedHandle2Volume, MountPath: "/handle-2/path"},
							}))
						})

						Context("when LookupVolume returns an error", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeBaggageclaimClient.LookupVolumeReturns(nil, false, disaster)
							})

							It("returns the error on lookup", func() {
								Expect(findErr).To(Equal(disaster))
							})
						})

						Context("when Build returns an error", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeVolumeFactory.BuildReturns(nil, false, disaster)
							})

							It("returns the error on lookup", func() {
								Expect(findErr).To(Equal(disaster))
							})
						})

						Context("when there is no baggageclaim", func() {
							BeforeEach(func() {
								gardenWorker = NewGardenWorker(
									fakeGardenClient,
									nil,
									fakeVolumeClient,
									nil,
									fakeImageFetcher,
									fakeGardenWorkerDB,
									fakeWorkerProvider,
									fakeClock,
									activeContainers,
									resourceTypes,
									platform,
									tags,
									workerName,
									workerStartTime,
									httpProxyURL,
									httpsProxyURL,
									noProxy,
								)
							})

							It("returns an empty slice", func() {
								Expect(foundContainer.Volumes()).To(BeEmpty())
							})
						})
					})

					Describe("Release", func() {
						It("releases the container's volumes once and only once", func() {
							foundContainer.Release(FinalTTL(time.Minute))
							Expect(expectedHandle1Volume.ReleaseCallCount()).To(Equal(1))
							Expect(expectedHandle1Volume.ReleaseArgsForCall(0)).To(Equal(FinalTTL(time.Minute)))
							Expect(expectedHandle2Volume.ReleaseCallCount()).To(Equal(1))
							Expect(expectedHandle2Volume.ReleaseArgsForCall(0)).To(Equal(FinalTTL(time.Minute)))

							foundContainer.Release(FinalTTL(time.Hour))
							Expect(expectedHandle1Volume.ReleaseCallCount()).To(Equal(1))
							Expect(expectedHandle2Volume.ReleaseCallCount()).To(Equal(1))
						})
					})
				})

				Context("when the concourse:volumes property is not present", func() {
					BeforeEach(func() {
						fakeContainer.PropertiesReturns(garden.Properties{}, nil)
					})

					Describe("Volumes", func() {
						It("returns an empty slice", func() {
							Expect(foundContainer.Volumes()).To(BeEmpty())
						})
					})
				})

				Context("when the user property is present", func() {
					var (
						actualSpec garden.ProcessSpec
						actualIO   garden.ProcessIO
					)

					BeforeEach(func() {
						actualSpec = garden.ProcessSpec{
							Path: "some-path",
							Args: []string{"some", "args"},
							Env:  []string{"some=env"},
							Dir:  "some-dir",
						}

						actualIO = garden.ProcessIO{}

						fakeContainer.PropertiesReturns(garden.Properties{"user": "maverick"}, nil)
					})

					JustBeforeEach(func() {
						foundContainer.Run(actualSpec, actualIO)
					})

					Describe("Run", func() {
						It("calls Run() on the garden container and injects the user", func() {
							Expect(fakeContainer.RunCallCount()).To(Equal(1))
							spec, io := fakeContainer.RunArgsForCall(0)
							Expect(spec).To(Equal(garden.ProcessSpec{
								Path: "some-path",
								Args: []string{"some", "args"},
								Env:  []string{"some=env"},
								Dir:  "some-dir",
								User: "maverick",
							}))
							Expect(io).To(Equal(garden.ProcessIO{}))
						})
					})
				})

				Context("when the user property is not present", func() {
					var (
						actualSpec garden.ProcessSpec
						actualIO   garden.ProcessIO
					)

					BeforeEach(func() {
						actualSpec = garden.ProcessSpec{
							Path: "some-path",
							Args: []string{"some", "args"},
							Env:  []string{"some=env"},
							Dir:  "some-dir",
						}

						actualIO = garden.ProcessIO{}

						fakeContainer.PropertiesReturns(garden.Properties{"user": ""}, nil)
					})

					JustBeforeEach(func() {
						foundContainer.Run(actualSpec, actualIO)
					})

					Describe("Run", func() {
						It("calls Run() on the garden container and injects the default user", func() {
							Expect(fakeContainer.RunCallCount()).To(Equal(1))
							spec, io := fakeContainer.RunArgsForCall(0)
							Expect(spec).To(Equal(garden.ProcessSpec{
								Path: "some-path",
								Args: []string{"some", "args"},
								Env:  []string{"some=env"},
								Dir:  "some-dir",
								User: "root",
							}))
							Expect(io).To(Equal(garden.ProcessIO{}))
							Expect(fakeContainer.RunCallCount()).To(Equal(1))
						})
					})
				})
			})
		})

		Context("when the gardenClient returns garden.ContainerNotFoundError", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "some-handle"})
			})

			It("returns false and no error", func() {
				_, found, err := gardenWorker.LookupContainer(logger, handle)
				Expect(err).ToNot(HaveOccurred())

				Expect(found).To(BeFalse())
			})
		})

		Context("when the gardenClient returns an error", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = fmt.Errorf("container not found")
				fakeGardenClient.LookupReturns(nil, expectedErr)
			})

			It("returns nil and forwards the error", func() {
				foundContainer, _, err := gardenWorker.LookupContainer(logger, handle)
				Expect(err).To(Equal(expectedErr))

				Expect(foundContainer).To(BeNil())
			})
		})
	})

	Describe("FindContainerForIdentifier", func() {
		var (
			id Identifier

			foundContainer Container
			found          bool
			lookupErr      error
		)

		BeforeEach(func() {
			id = Identifier{
				ResourceID: 1234,
			}
		})

		JustBeforeEach(func() {
			foundContainer, found, lookupErr = gardenWorker.FindContainerForIdentifier(logger, id)
		})

		Context("when the container can be found", func() {
			var (
				fakeContainer *gfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("provider-handle")

				fakeWorkerProvider.FindContainerForIdentifierReturns(db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Handle: "provider-handle",
						},
					},
				}, true, nil)

				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).NotTo(HaveOccurred())
			})

			It("looks for containers with matching properties via the Garden client", func() {
				Expect(fakeWorkerProvider.FindContainerForIdentifierCallCount()).To(Equal(1))
				Expect(fakeWorkerProvider.FindContainerForIdentifierArgsForCall(0)).To(Equal(id))

				Expect(fakeGardenClient.LookupCallCount()).To(Equal(1))
				lookupHandle := fakeGardenClient.LookupArgsForCall(0)

				Expect(lookupHandle).To(Equal("provider-handle"))
			})

			Describe("the found container", func() {
				BeforeEach(func() {
					fakeGardenWorkerDB.GetContainerReturns(db.SavedContainer{
						TTL: 5 * time.Minute,
					}, true, nil)
				})

				It("can be destroyed", func() {
					err := foundContainer.Destroy()
					Expect(err).NotTo(HaveOccurred())

					By("destroying via garden")
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
					Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("provider-handle"))

					By("no longer heartbeating")
					fakeClock.Increment(30 * time.Second)
					Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
				})

				It("performs an initial heartbeat synchronously", func() {
					Expect(fakeContainer.SetGraceTimeCallCount()).To(Equal(1))
					Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).To(Equal(1))
				})

				Describe("every 30 seconds", func() {
					It("heartbeats to the database and the container", func() {
						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
						Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(5 * time.Minute))

						Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
						handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(5 * time.Minute))

						fakeClock.Increment(30 * time.Second)

						Eventually(fakeContainer.SetGraceTimeCallCount).Should(Equal(3))
						Expect(fakeContainer.SetGraceTimeArgsForCall(2)).To(Equal(5 * time.Minute))

						Eventually(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(3))
						handle, interval = fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(2)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(5 * time.Minute))
					})
				})

				Describe("releasing", func() {
					It("sets a final ttl on the container and stops heartbeating", func() {
						foundContainer.Release(FinalTTL(30 * time.Minute))

						Expect(fakeContainer.SetGraceTimeCallCount()).Should(Equal(2))
						Expect(fakeContainer.SetGraceTimeArgsForCall(1)).To(Equal(30 * time.Minute))

						Expect(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount()).Should(Equal(2))
						handle, interval := fakeGardenWorkerDB.UpdateExpiresAtOnContainerArgsForCall(1)
						Expect(handle).To(Equal("provider-handle"))
						Expect(interval).To(Equal(30 * time.Minute))

						fakeClock.Increment(30 * time.Second)

						Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(2))
						Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(2))
					})

					Context("with no final ttl", func() {
						It("does not perform a final heartbeat", func() {
							foundContainer.Release(nil)

							Consistently(fakeContainer.SetGraceTimeCallCount).Should(Equal(1))
							Consistently(fakeGardenWorkerDB.UpdateExpiresAtOnContainerCallCount).Should(Equal(1))
						})
					})
				})

				It("can be released multiple times", func() {
					foundContainer.Release(nil)

					Expect(func() {
						foundContainer.Release(nil)
					}).NotTo(Panic())
				})
			})
		})

		Context("when no containers are found", func() {
			BeforeEach(func() {
				fakeWorkerProvider.FindContainerForIdentifierReturns(db.SavedContainer{}, false, nil)
			})

			It("returns that the container could not be found", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when finding the containers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeWorkerProvider.FindContainerForIdentifierReturns(db.SavedContainer{}, false, disaster)
			})

			It("returns the error", func() {
				Expect(lookupErr).To(Equal(disaster))
			})
		})

		Context("when the container cannot be found", func() {
			BeforeEach(func() {
				containerToReturn := db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Handle: "handle",
						},
					},
				}

				fakeWorkerProvider.FindContainerForIdentifierReturns(containerToReturn, true, nil)
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "handle"})
			})

			It("expires the container and returns false and no error", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundContainer).To(BeNil())

				expiredHandle := fakeWorkerProvider.ReapContainerArgsForCall(0)
				Expect(expiredHandle).To(Equal("handle"))
			})
		})

		Context("when looking up the container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				containerToReturn := db.SavedContainer{
					Container: db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Handle: "handle",
						},
					},
				}

				fakeWorkerProvider.FindContainerForIdentifierReturns(containerToReturn, true, nil)
				fakeGardenClient.LookupReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(lookupErr).To(Equal(disaster))
			})
		})
	})

	Describe("Satisfying", func() {
		var (
			spec WorkerSpec

			satisfyingWorker Worker
			satisfyingErr    error

			customTypes atc.ResourceTypes
		)

		BeforeEach(func() {
			spec = WorkerSpec{}
			customTypes = atc.ResourceTypes{
				{
					Name:   "custom-type-b",
					Type:   "custom-type-a",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-a",
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-c",
					Type:   "custom-type-b",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "custom-type-d",
					Type:   "custom-type-b",
					Source: atc.Source{"some": "source"},
				},
				{
					Name:   "unknown-custom-type",
					Type:   "unknown-base-type",
					Source: atc.Source{"some": "source"},
				},
			}
		})

		JustBeforeEach(func() {
			gardenWorker = NewGardenWorker(
				fakeGardenClient,
				fakeBaggageclaimClient,
				fakeVolumeClient,
				fakeVolumeFactory,
				fakeImageFetcher,
				fakeGardenWorkerDB,
				fakeWorkerProvider,
				fakeClock,
				activeContainers,
				resourceTypes,
				platform,
				tags,
				workerName,
				workerStartTime,
				httpProxyURL,
				httpsProxyURL,
				noProxy,
			)

			satisfyingWorker, satisfyingErr = gardenWorker.Satisfying(spec, customTypes)
		})

		Context("when the platform is compatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-platform"
			})

			Context("when no tags are specified", func() {
				BeforeEach(func() {
					spec.Tags = nil
				})

				It("returns ErrIncompatiblePlatform", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})

			Context("when the worker has no tags", func() {
				BeforeEach(func() {
					tags = []string{}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrMismatchedTags", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})
		})

		Context("when the platform is incompatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-bogus-platform"
			})

			It("returns ErrIncompatiblePlatform", func() {
				Expect(satisfyingErr).To(Equal(ErrIncompatiblePlatform))
			})
		})

		Context("when the resource type is supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "some-resource"
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns the worker", func() {
					Expect(satisfyingWorker).To(Equal(gardenWorker))
				})

				It("returns no error", func() {
					Expect(satisfyingErr).NotTo(HaveOccurred())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrMismatchedTags", func() {
					Expect(satisfyingErr).To(Equal(ErrMismatchedTags))
				})
			})
		})

		Context("when the resource type is a custom type supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "custom-type-c"
				spec.Tags = []string{"some", "tags"}
			})

			It("returns the worker", func() {
				Expect(satisfyingWorker).To(Equal(gardenWorker))
			})

			It("returns no error", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
			})
		})

		Context("when the resource type is a custom type that overrides one supported by the worker", func() {
			BeforeEach(func() {
				customTypes = append(customTypes, atc.ResourceType{
					Name:   "some-resource",
					Type:   "some-resource",
					Source: atc.Source{"some": "source"},
				})

				spec.ResourceType = "some-resource"
				spec.Tags = []string{"some", "tags"}
			})

			It("returns the worker", func() {
				Expect(satisfyingWorker).To(Equal(gardenWorker))
			})

			It("returns no error", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
			})
		})

		Context("when the resource type is a custom type that results in a circular dependency", func() {
			BeforeEach(func() {
				customTypes = append(customTypes, atc.ResourceType{
					Name:   "circle-a",
					Type:   "circle-b",
					Source: atc.Source{"some": "source"},
				}, atc.ResourceType{
					Name:   "circle-b",
					Type:   "circle-c",
					Source: atc.Source{"some": "source"},
				}, atc.ResourceType{
					Name:   "circle-c",
					Type:   "circle-a",
					Source: atc.Source{"some": "source"},
				})

				spec.ResourceType = "circle-a"
				spec.Tags = []string{"some", "tags"}
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when the resource type is a custom type not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "unknown-custom-type"
				spec.Tags = []string{"some", "tags"}
			})

			It("returns ErrUnsupportedResourceType", func() {
				Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
			})
		})

		Context("when the type is not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "some-other-resource"
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns ErrUnsupportedResourceType", func() {
					Expect(satisfyingErr).To(Equal(ErrUnsupportedResourceType))
				})
			})
		})
	})
})

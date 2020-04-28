package worker_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/gclient/gclientfakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/cppforlife/go-semi-semantic/version"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker", func() {
	var (
		logger                    *lagertest.TestLogger
		fakeVolumeClient          *workerfakes.FakeVolumeClient
		activeContainers          int
		resourceTypes             []atc.WorkerResourceType
		platform                  string
		tags                      atc.Tags
		teamID                    int
		ephemeral                 bool
		workerName                string
		gardenWorker              Worker
		workerVersion             string
		fakeGardenClient          *gclientfakes.FakeClient
		fakeImageFactory          *workerfakes.FakeImageFactory
		fakeImage                 *workerfakes.FakeImage
		fakeDBWorker              *dbfakes.FakeWorker
		fakeDBVolumeRepository    *dbfakes.FakeVolumeRepository
		fakeDBTeamFactory         *dbfakes.FakeTeamFactory
		fakeDBTeam                *dbfakes.FakeTeam
		fakeCreatingContainer     *dbfakes.FakeCreatingContainer
		fakeCreatedContainer      *dbfakes.FakeCreatedContainer
		fakeGardenContainer       *gclientfakes.FakeContainer
		fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate
		fakeBaggageclaimClient    *baggageclaimfakes.FakeClient
		fakeFetcher               *workerfakes.FakeFetcher

		fakeLocalInput    *workerfakes.FakeInputSource
		fakeRemoteInput   *workerfakes.FakeInputSource
		fakeRemoteInputAS *workerfakes.FakeStreamableArtifactSource

		fakeBindMount *workerfakes.FakeBindMountSource

		fakeRemoteInputContainerVolume *workerfakes.FakeVolume
		fakeLocalVolume                *workerfakes.FakeVolume
		fakeOutputVolume               *workerfakes.FakeVolume
		fakeLocalCOWVolume             *workerfakes.FakeVolume

		ctx                context.Context
		containerSpec      ContainerSpec
		fakeContainerOwner *dbfakes.FakeContainerOwner
		containerMetadata  db.ContainerMetadata

		stubbedVolumes   map[string]*workerfakes.FakeVolume
		volumeSpecs      map[string]VolumeSpec
		atcResourceTypes atc.VersionedResourceTypes

		findOrCreateErr       error
		findOrCreateContainer Container
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeVolumeClient = new(workerfakes.FakeVolumeClient)
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
		teamID = 17
		ephemeral = true
		workerName = "some-worker"
		workerVersion = "1.2.3"
		fakeDBWorker = new(dbfakes.FakeWorker)

		fakeGardenClient = new(gclientfakes.FakeClient)
		fakeImageFactory = new(workerfakes.FakeImageFactory)
		fakeImage = new(workerfakes.FakeImage)
		fakeImageFactory.GetImageReturns(fakeImage, nil)
		fakeFetcher = new(workerfakes.FakeFetcher)

		fakeCreatingContainer = new(dbfakes.FakeCreatingContainer)
		fakeCreatingContainer.HandleReturns("some-handle")
		fakeCreatedContainer = new(dbfakes.FakeCreatedContainer)
		fakeCreatedContainer.HandleReturns("some-handle")

		fakeDBVolumeRepository = new(dbfakes.FakeVolumeRepository)

		fakeDBTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeDBTeam = new(dbfakes.FakeTeam)
		fakeDBTeamFactory.GetByIDReturns(fakeDBTeam)

		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)

		fakeBaggageclaimClient = new(baggageclaimfakes.FakeClient)

		fakeLocalInput = new(workerfakes.FakeInputSource)
		fakeLocalInput.DestinationPathReturns("/some/work-dir/local-input")
		fakeLocalInputAS := new(workerfakes.FakeArtifactSource)
		fakeLocalVolume = new(workerfakes.FakeVolume)
		fakeLocalVolume.PathReturns("/fake/local/volume")
		fakeLocalVolume.COWStrategyReturns(baggageclaim.COWStrategy{
			Parent: new(baggageclaimfakes.FakeVolume),
		})
		fakeLocalInputAS.ExistsOnReturns(fakeLocalVolume, true, nil)
		fakeLocalInput.SourceReturns(fakeLocalInputAS)

		fakeBindMount = new(workerfakes.FakeBindMountSource)
		fakeBindMount.VolumeOnReturns(garden.BindMount{
			SrcPath: "some/source",
			DstPath: "some/destination",
			Mode:    garden.BindMountModeRO,
		}, true, nil)

		fakeRemoteInput = new(workerfakes.FakeInputSource)
		fakeRemoteInput.DestinationPathReturns("/some/work-dir/remote-input")
		fakeRemoteInputAS = new(workerfakes.FakeStreamableArtifactSource)
		fakeRemoteInputAS.ExistsOnReturns(nil, false, nil)
		fakeRemoteInput.SourceReturns(fakeRemoteInputAS)

		fakeScratchVolume := new(workerfakes.FakeVolume)
		fakeScratchVolume.PathReturns("/fake/scratch/volume")

		fakeWorkdirVolume := new(workerfakes.FakeVolume)
		fakeWorkdirVolume.PathReturns("/fake/work-dir/volume")

		fakeOutputVolume = new(workerfakes.FakeVolume)
		fakeOutputVolume.PathReturns("/fake/output/volume")

		fakeLocalCOWVolume = new(workerfakes.FakeVolume)
		fakeLocalCOWVolume.PathReturns("/fake/local/cow/volume")

		fakeRemoteInputContainerVolume = new(workerfakes.FakeVolume)
		fakeRemoteInputContainerVolume.PathReturns("/fake/remote/input/container/volume")

		stubbedVolumes = map[string]*workerfakes.FakeVolume{
			"/scratch":                    fakeScratchVolume,
			"/some/work-dir":              fakeWorkdirVolume,
			"/some/work-dir/local-input":  fakeLocalCOWVolume,
			"/some/work-dir/remote-input": fakeRemoteInputContainerVolume,
			"/some/work-dir/output":       fakeOutputVolume,
		}

		volumeSpecs = map[string]VolumeSpec{}

		fakeVolumeClient.FindOrCreateCOWVolumeForContainerStub = func(logger lager.Logger, volumeSpec VolumeSpec, creatingContainer db.CreatingContainer, volume Volume, teamID int, mountPath string) (Volume, error) {
			Expect(volume).To(Equal(fakeLocalVolume))

			volume, found := stubbedVolumes[mountPath]
			if !found {
				panic("unknown container volume: " + mountPath)
			}

			volumeSpecs[mountPath] = volumeSpec

			return volume, nil
		}

		fakeVolumeClient.FindOrCreateVolumeForContainerStub = func(logger lager.Logger, volumeSpec VolumeSpec, creatingContainer db.CreatingContainer, teamID int, mountPath string) (Volume, error) {
			volume, found := stubbedVolumes[mountPath]
			if !found {
				panic("unknown container volume: " + mountPath)
			}

			volumeSpecs[mountPath] = volumeSpec

			return volume, nil
		}
		ctx = context.Background()

		fakeContainerOwner = new(dbfakes.FakeContainerOwner)

		fakeImage.FetchForContainerReturns(FetchedImage{
			Metadata: ImageMetadata{
				Env: []string{"IMAGE=ENV"},
			},
			URL: "some-image-url",
		}, nil)
		containerMetadata = db.ContainerMetadata{
			StepName: "some-step",
		}

		cpu := uint64(1024)
		memory := uint64(1024)
		containerSpec = ContainerSpec{
			TeamID: 73410,

			ImageSpec: ImageSpec{
				ImageResource: &ImageResource{
					Type:   "registry-image",
					Source: atc.Source{"some": "super-secret-image"},
				},
			},

			User: "some-user",
			Env:  []string{"SOME=ENV"},

			Dir: "/some/work-dir",

			Inputs: []InputSource{
				fakeLocalInput,
				fakeRemoteInput,
			},

			Outputs: OutputPaths{
				"some-output": "/some/work-dir/output",
			},
			BindMounts: []BindMountSource{
				fakeBindMount,
			},
			Limits: ContainerLimits{
				CPU:    &cpu,
				Memory: &memory,
			},
		}

		atcResourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Type:   "some-type",
					Source: atc.Source{"some": "super-secret-source"},
				},
				Version: atc.Version{"some": "version"},
			},
		}

		fakeGardenContainer = new(gclientfakes.FakeContainer)
		fakeGardenClient.CreateReturns(fakeGardenContainer, nil)
	})

	JustBeforeEach(func() {
		fakeDBWorker.ActiveContainersReturns(activeContainers)
		fakeDBWorker.ResourceTypesReturns(resourceTypes)
		fakeDBWorker.PlatformReturns(platform)
		fakeDBWorker.TagsReturns(tags)
		fakeDBWorker.EphemeralReturns(ephemeral)
		fakeDBWorker.TeamIDReturns(teamID)
		fakeDBWorker.NameReturns(workerName)
		fakeDBWorker.VersionReturns(&workerVersion)
		fakeDBWorker.HTTPProxyURLReturns("http://proxy.com")
		fakeDBWorker.HTTPSProxyURLReturns("https://proxy.com")
		fakeDBWorker.NoProxyReturns("http://noproxy.com")

		gardenWorker = NewGardenWorker(
			fakeGardenClient,
			fakeDBVolumeRepository,
			fakeVolumeClient,
			fakeImageFactory,
			fakeFetcher,
			fakeDBTeamFactory,
			fakeDBWorker,
			0,
		)
	})

	Describe("IsVersionCompatible", func() {
		It("is compatible when versions are the same", func() {
			requiredVersion := version.MustNewVersionFromString("1.2.3")
			Expect(
				gardenWorker.IsVersionCompatible(logger, requiredVersion),
			).To(BeTrue())
		})

		It("is not compatible when versions are different in major version", func() {
			requiredVersion := version.MustNewVersionFromString("2.2.3")
			Expect(
				gardenWorker.IsVersionCompatible(logger, requiredVersion),
			).To(BeFalse())
		})

		It("is compatible when worker minor version is newer", func() {
			requiredVersion := version.MustNewVersionFromString("1.1.3")
			Expect(
				gardenWorker.IsVersionCompatible(logger, requiredVersion),
			).To(BeTrue())
		})

		It("is not compatible when worker minor version is older", func() {
			requiredVersion := version.MustNewVersionFromString("1.3.3")
			Expect(
				gardenWorker.IsVersionCompatible(logger, requiredVersion),
			).To(BeFalse())
		})

		Context("when worker version is empty", func() {
			BeforeEach(func() {
				workerVersion = ""
			})

			It("is not compatible", func() {
				requiredVersion := version.MustNewVersionFromString("1.2.3")
				Expect(
					gardenWorker.IsVersionCompatible(logger, requiredVersion),
				).To(BeFalse())
			})
		})

		Context("when worker version does not have minor version", func() {
			BeforeEach(func() {
				workerVersion = "1"
			})

			It("is compatible when it is the same", func() {
				requiredVersion := version.MustNewVersionFromString("1")
				Expect(
					gardenWorker.IsVersionCompatible(logger, requiredVersion),
				).To(BeTrue())
			})

			It("is not compatible when it is different", func() {
				requiredVersion := version.MustNewVersionFromString("2")
				Expect(
					gardenWorker.IsVersionCompatible(logger, requiredVersion),
				).To(BeFalse())
			})

			It("is not compatible when compared version has minor vesion", func() {
				requiredVersion := version.MustNewVersionFromString("1.2")
				Expect(
					gardenWorker.IsVersionCompatible(logger, requiredVersion),
				).To(BeFalse())
			})
		})
	})

	Describe("FindCreatedContainerByHandle", func() {
		var (
			foundContainer Container
			findErr        error
			found          bool
		)

		JustBeforeEach(func() {
			foundContainer, found, findErr = gardenWorker.FindContainerByHandle(logger, 42, "some-container-handle")
		})
		Context("when the gardenClient returns a container and no error", func() {
			var (
				fakeContainer *gclientfakes.FakeContainer
			)

			BeforeEach(func() {
				fakeContainer = new(gclientfakes.FakeContainer)
				fakeContainer.HandleReturns("provider-handle")

				fakeDBVolumeRepository.FindVolumesForContainerReturns([]db.CreatedVolume{}, nil)

				fakeDBTeam.FindCreatedContainerByHandleReturns(fakeCreatedContainer, true, nil)
				fakeGardenClient.LookupReturns(fakeContainer, nil)
			})

			It("returns the container", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundContainer.Handle()).To(Equal(fakeContainer.Handle()))
			})

			Describe("the found container", func() {
				It("can be destroyed", func() {
					err := foundContainer.Destroy()
					Expect(err).NotTo(HaveOccurred())

					By("destroying via garden")
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
					actualHandle := fakeGardenClient.DestroyArgsForCall(0)
					Expect(actualHandle).To(Equal("provider-handle"))
				})
			})

			Context("when the concourse:volumes property is present", func() {
				var (
					expectedHandle1Volume *workerfakes.FakeVolume
					expectedHandle2Volume *workerfakes.FakeVolume
				)

				BeforeEach(func() {
					expectedHandle1Volume = new(workerfakes.FakeVolume)
					expectedHandle2Volume = new(workerfakes.FakeVolume)

					expectedHandle1Volume.HandleReturns("handle-1")
					expectedHandle2Volume.HandleReturns("handle-2")

					expectedHandle1Volume.PathReturns("/handle-1/path")
					expectedHandle2Volume.PathReturns("/handle-2/path")

					fakeVolumeClient.LookupVolumeStub = func(logger lager.Logger, handle string) (Volume, bool, error) {
						if handle == "handle-1" {
							return expectedHandle1Volume, true, nil
						} else if handle == "handle-2" {
							return expectedHandle2Volume, true, nil
						} else {
							panic("unknown handle: " + handle)
						}
					}

					dbVolume1 := new(dbfakes.FakeCreatedVolume)
					dbVolume2 := new(dbfakes.FakeCreatedVolume)
					fakeDBVolumeRepository.FindVolumesForContainerReturns([]db.CreatedVolume{dbVolume1, dbVolume2}, nil)
					dbVolume1.HandleReturns("handle-1")
					dbVolume2.HandleReturns("handle-2")
					dbVolume1.PathReturns("/handle-1/path")
					dbVolume2.PathReturns("/handle-2/path")
				})

				Describe("VolumeMounts", func() {
					It("returns all bound volumes based on properties on the container", func() {
						Expect(findErr).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
						Expect(foundContainer.VolumeMounts()).To(ConsistOf([]VolumeMount{
							{Volume: expectedHandle1Volume, MountPath: "/handle-1/path"},
							{Volume: expectedHandle2Volume, MountPath: "/handle-2/path"},
						}))
					})

					Context("when LookupVolume returns an error", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVolumeClient.LookupVolumeReturns(nil, false, disaster)
						})

						It("returns the error on lookup", func() {
							Expect(findErr).To(Equal(disaster))
						})
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
					foundContainer.Run(context.TODO(), actualSpec, actualIO)
				})

				Describe("Run", func() {
					It("calls Run() on the garden container and injects the user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
						_, spec, io := fakeContainer.RunArgsForCall(0)
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
					foundContainer.Run(context.TODO(), actualSpec, actualIO)
				})

				Describe("Run", func() {
					It("calls Run() on the garden container and injects the default user", func() {
						Expect(fakeContainer.RunCallCount()).To(Equal(1))
						_, spec, io := fakeContainer.RunArgsForCall(0)
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

		Context("when the gardenClient returns garden.ContainerNotFoundError", func() {
			BeforeEach(func() {
				fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "some-handle"})
			})
			It("returns false and no error", func() {
				Expect(findErr).ToNot(HaveOccurred())
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
				Expect(findErr).To(Equal(expectedErr))

				Expect(foundContainer).To(BeNil())
			})
		})
	})

	Describe("CreateVolume", func() {
		var (
			fakeVolume *workerfakes.FakeVolume
			volume     Volume
			err        error
		)

		BeforeEach(func() {
			fakeVolume = new(workerfakes.FakeVolume)
			fakeVolumeClient.CreateVolumeReturns(fakeVolume, nil)
		})

		JustBeforeEach(func() {
			volume, err = gardenWorker.CreateVolume(logger, VolumeSpec{}, 42, db.VolumeTypeArtifact)
		})

		It("calls the volume client", func() {
			Expect(fakeVolumeClient.CreateVolumeCallCount()).To(Equal(1))

			Expect(err).ToNot(HaveOccurred())
			Expect(volume).To(Equal(fakeVolume))
		})
	})

	Describe("Satisfies", func() {
		var (
			spec WorkerSpec

			satisfies bool

			customTypes atc.VersionedResourceTypes
		)

		BeforeEach(func() {

			customTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-b",
						Type:   "custom-type-a",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-a",
						Type:   "some-resource",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-c",
						Type:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-d",
						Type:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "unknown-custom-type",
						Type:   "unknown-base-type",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
			}

			spec = WorkerSpec{
				Tags:          []string{"some", "tags"},
				TeamID:        teamID,
				ResourceTypes: customTypes,
			}
		})

		JustBeforeEach(func() {
			satisfies = gardenWorker.Satisfies(logger, spec)
		})

		Context("when the platform is compatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-platform"
			})

			Context("when no tags are specified", func() {
				BeforeEach(func() {
					spec.Tags = nil
				})

				It("returns false", func() {
					Expect(satisfies).To(BeFalse())
				})
			})

			Context("when the worker has no tags", func() {
				BeforeEach(func() {
					tags = []string{}
					spec.Tags = []string{}
				})

				It("returns true", func() {
					Expect(satisfies).To(BeTrue())
				})
			})

			Context("when all of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some", "tags"}
				})

				It("returns true", func() {
					Expect(satisfies).To(BeTrue())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns true", func() {
					Expect(satisfies).To(BeTrue())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns false", func() {
					Expect(satisfies).To(BeFalse())
				})
			})
		})

		Context("when the platform is incompatible", func() {
			BeforeEach(func() {
				spec.Platform = "some-bogus-platform"
			})

			It("returns false", func() {
				Expect(satisfies).To(BeFalse())
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

				It("returns true", func() {
					Expect(satisfies).To(BeTrue())
				})
			})

			Context("when some of the requested tags are present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"some"}
				})

				It("returns true", func() {
					Expect(satisfies).To(BeTrue())
				})
			})

			Context("when any of the requested tags are not present", func() {
				BeforeEach(func() {
					spec.Tags = []string{"bogus", "tags"}
				})

				It("returns false", func() {
					Expect(satisfies).To(BeFalse())
				})
			})
		})

		Context("when the resource type is a custom type supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "custom-type-c"
			})

			It("returns true", func() {
				Expect(satisfies).To(BeTrue())
			})
		})

		Context("when the resource type is a custom type that overrides one supported by the worker", func() {
			BeforeEach(func() {

				customTypes = atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Name:   "some-resource",
							Type:   "some-resource",
							Source: atc.Source{"some": "source"},
						},
						Version: atc.Version{"some": "version"},
					},
				}

				spec.ResourceType = "some-resource"
			})

			It("returns true", func() {
				Expect(satisfies).To(BeTrue())
			})
		})

		Context("when the resource type is a custom type that results in a circular dependency", func() {
			BeforeEach(func() {

				customTypes = atc.VersionedResourceTypes{
					atc.VersionedResourceType{
						ResourceType: atc.ResourceType{
							Name:   "circle-a",
							Type:   "circle-b",
							Source: atc.Source{"some": "source"},
						},
						Version: atc.Version{"some": "version"},
					}, atc.VersionedResourceType{
						ResourceType: atc.ResourceType{
							Name:   "circle-b",
							Type:   "circle-c",
							Source: atc.Source{"some": "source"},
						},
						Version: atc.Version{"some": "version"},
					}, atc.VersionedResourceType{
						ResourceType: atc.ResourceType{
							Name:   "circle-c",
							Type:   "circle-a",
							Source: atc.Source{"some": "source"},
						},
						Version: atc.Version{"some": "version"},
					},
				}

				spec.ResourceType = "circle-a"
			})

			It("returns false", func() {
				Expect(satisfies).To(BeFalse())
			})
		})

		Context("when the resource type is a custom type not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "unknown-custom-type"
			})

			It("returns false", func() {
				Expect(satisfies).To(BeFalse())
			})
		})

		Context("when the type is not supported by the worker", func() {
			BeforeEach(func() {
				spec.ResourceType = "some-other-resource"
			})

			It("returns false", func() {
				Expect(satisfies).To(BeFalse())
			})
		})

		Context("when spec specifies team", func() {
			BeforeEach(func() {
				teamID = 123
				spec.TeamID = teamID
			})

			Context("when worker belongs to same team", func() {
				It("returns true", func() {
					Expect(satisfies).To(BeTrue())
				})
			})

			Context("when worker belongs to different team", func() {
				BeforeEach(func() {
					teamID = 777
				})

				It("returns false", func() {
					Expect(satisfies).To(BeFalse())
				})
			})

			Context("when worker does not belong to any team", func() {
				It("returns true", func() {
					Expect(satisfies).To(BeTrue())
				})
			})
		})

		Context("when spec does not specify a team", func() {
			Context("when worker belongs to no team", func() {
				BeforeEach(func() {
					teamID = 0
				})

				It("returns true", func() {
					Expect(satisfies).To(BeTrue())
				})
			})

			Context("when worker belongs to any team", func() {
				BeforeEach(func() {
					teamID = 555
				})

				It("returns false", func() {
					Expect(satisfies).To(BeFalse())
				})
			})
		})
	})

	Describe("FindOrCreateContainer", func() {
		CertsVolumeExists := func() {
			fakeCertsVolume := new(baggageclaimfakes.FakeVolume)
			fakeBaggageclaimClient.LookupVolumeReturns(fakeCertsVolume, true, nil)
		}

		JustBeforeEach(func() {
			findOrCreateContainer, findOrCreateErr = gardenWorker.FindOrCreateContainer(
				ctx,
				logger,
				fakeImageFetchingDelegate,
				fakeContainerOwner,
				containerMetadata,
				containerSpec,
				atcResourceTypes,
			)
		})
		disasterErr := errors.New("disaster")

		Context("when container exists in database in creating state", func() {
			BeforeEach(func() {
				fakeDBWorker.FindContainerReturns(fakeCreatingContainer, nil, nil)
			})

			It("does not create a new db container", func() {
				Expect(fakeDBWorker.CreateContainerCallCount()).To(Equal(0))
			})

			Context("when container exists in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(fakeGardenContainer, nil)
				})

				It("marks container as created", func() {
					Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(1))
				})

				It("returns worker container", func() {
					Expect(findOrCreateContainer).ToNot(BeNil())
				})
			})

			Context("when container does not exist in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{})
				})
				BeforeEach(CertsVolumeExists)

				It("creates container in garden", func() {
					Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
				})

				It("marks container as created", func() {
					Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(1))
				})

				It("returns worker container", func() {
					Expect(findOrCreateContainer).ToNot(BeNil())
				})

				It("creates the container in garden with the input and output volumes in alphabetical order", func() {
					Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

					actualSpec := fakeGardenClient.CreateArgsForCall(0)
					Expect(actualSpec).To(Equal(garden.ContainerSpec{
						Handle:     "some-handle",
						RootFSPath: "some-image-url",
						Properties: garden.Properties{"user": "some-user"},
						BindMounts: []garden.BindMount{
							{
								SrcPath: "some/source",
								DstPath: "some/destination",
								Mode:    garden.BindMountModeRO,
							},
							{
								SrcPath: "/fake/scratch/volume",
								DstPath: "/scratch",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/fake/work-dir/volume",
								DstPath: "/some/work-dir",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/fake/local/cow/volume",
								DstPath: "/some/work-dir/local-input",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/fake/output/volume",
								DstPath: "/some/work-dir/output",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/fake/remote/input/container/volume",
								DstPath: "/some/work-dir/remote-input",
								Mode:    garden.BindMountModeRW,
							},
						},
						Limits: garden.Limits{
							CPU:    garden.CPULimits{LimitInShares: 1024},
							Memory: garden.MemoryLimits{LimitInBytes: 1024},
						},
						Env: []string{
							"IMAGE=ENV",
							"SOME=ENV",
							"http_proxy=http://proxy.com",
							"https_proxy=https://proxy.com",
							"no_proxy=http://noproxy.com",
						},
					}))
				})

				Context("when the input and output destination paths overlap", func() {
					var (
						fakeRemoteInputUnderInput    *workerfakes.FakeInputSource
						fakeRemoteInputUnderInputAS  *workerfakes.FakeArtifactSource
						fakeRemoteInputUnderOutput   *workerfakes.FakeInputSource
						fakeRemoteInputUnderOutputAS *workerfakes.FakeArtifactSource

						fakeOutputUnderInputVolume                *workerfakes.FakeVolume
						fakeOutputUnderOutputVolume               *workerfakes.FakeVolume
						fakeRemoteInputUnderInputContainerVolume  *workerfakes.FakeVolume
						fakeRemoteInputUnderOutputContainerVolume *workerfakes.FakeVolume
					)

					BeforeEach(func() {
						fakeRemoteInputUnderInput = new(workerfakes.FakeInputSource)
						fakeRemoteInputUnderInput.DestinationPathReturns("/some/work-dir/remote-input/other-input")
						fakeRemoteInputUnderInputAS = new(workerfakes.FakeArtifactSource)
						fakeRemoteInputUnderInputAS.ExistsOnReturns(nil, false, nil)
						fakeRemoteInputUnderInput.SourceReturns(fakeRemoteInputUnderInputAS)

						fakeRemoteInputUnderOutput = new(workerfakes.FakeInputSource)
						fakeRemoteInputUnderOutput.DestinationPathReturns("/some/work-dir/output/input")
						fakeRemoteInputUnderOutputAS = new(workerfakes.FakeArtifactSource)
						fakeRemoteInputUnderOutputAS.ExistsOnReturns(nil, false, nil)
						fakeRemoteInputUnderOutput.SourceReturns(fakeRemoteInputUnderOutputAS)

						fakeOutputUnderInputVolume = new(workerfakes.FakeVolume)
						fakeOutputUnderInputVolume.PathReturns("/fake/output/under/input/volume")
						fakeOutputUnderOutputVolume = new(workerfakes.FakeVolume)
						fakeOutputUnderOutputVolume.PathReturns("/fake/output/other-output/volume")

						fakeRemoteInputUnderInputContainerVolume = new(workerfakes.FakeVolume)
						fakeRemoteInputUnderInputContainerVolume.PathReturns("/fake/remote/input/other-input/container/volume")
						fakeRemoteInputUnderOutputContainerVolume = new(workerfakes.FakeVolume)
						fakeRemoteInputUnderOutputContainerVolume.PathReturns("/fake/output/input/container/volume")

						stubbedVolumes["/some/work-dir/remote-input/other-input"] = fakeRemoteInputUnderInputContainerVolume
						stubbedVolumes["/some/work-dir/output/input"] = fakeRemoteInputUnderOutputContainerVolume
						stubbedVolumes["/some/work-dir/output/other-output"] = fakeOutputUnderOutputVolume
						stubbedVolumes["/some/work-dir/local-input/output"] = fakeOutputUnderInputVolume
					})

					Context("outputs are nested under inputs", func() {
						BeforeEach(func() {
							containerSpec.Inputs = []InputSource{
								fakeLocalInput,
							}
							containerSpec.Outputs = OutputPaths{
								"some-output-under-input": "/some/work-dir/local-input/output",
							}
						})

						It("creates the container with correct bind mounts", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							actualSpec := fakeGardenClient.CreateArgsForCall(0)
							Expect(actualSpec).To(Equal(garden.ContainerSpec{
								Handle:     "some-handle",
								RootFSPath: "some-image-url",
								Properties: garden.Properties{"user": "some-user"},
								BindMounts: []garden.BindMount{
									{
										SrcPath: "some/source",
										DstPath: "some/destination",
										Mode:    garden.BindMountModeRO,
									},
									{
										SrcPath: "/fake/scratch/volume",
										DstPath: "/scratch",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/work-dir/volume",
										DstPath: "/some/work-dir",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/local/cow/volume",
										DstPath: "/some/work-dir/local-input",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/output/under/input/volume",
										DstPath: "/some/work-dir/local-input/output",
										Mode:    garden.BindMountModeRW,
									},
								},
								Limits: garden.Limits{
									CPU:    garden.CPULimits{LimitInShares: 1024},
									Memory: garden.MemoryLimits{LimitInBytes: 1024},
								},
								Env: []string{
									"IMAGE=ENV",
									"SOME=ENV",
									"http_proxy=http://proxy.com",
									"https_proxy=https://proxy.com",
									"no_proxy=http://noproxy.com",
								},
							}))
						})
					})

					Context("inputs are nested under inputs", func() {
						BeforeEach(func() {
							containerSpec.Inputs = []InputSource{
								fakeRemoteInput,
								fakeRemoteInputUnderInput,
							}
							containerSpec.Outputs = OutputPaths{}
						})

						It("creates the container with correct bind mounts", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							actualSpec := fakeGardenClient.CreateArgsForCall(0)
							Expect(actualSpec).To(Equal(garden.ContainerSpec{
								Handle:     "some-handle",
								RootFSPath: "some-image-url",
								Properties: garden.Properties{"user": "some-user"},
								BindMounts: []garden.BindMount{
									{
										SrcPath: "some/source",
										DstPath: "some/destination",
										Mode:    garden.BindMountModeRO,
									},
									{
										SrcPath: "/fake/scratch/volume",
										DstPath: "/scratch",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/work-dir/volume",
										DstPath: "/some/work-dir",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/remote/input/container/volume",
										DstPath: "/some/work-dir/remote-input",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/remote/input/other-input/container/volume",
										DstPath: "/some/work-dir/remote-input/other-input",
										Mode:    garden.BindMountModeRW,
									},
								},
								Limits: garden.Limits{
									CPU:    garden.CPULimits{LimitInShares: 1024},
									Memory: garden.MemoryLimits{LimitInBytes: 1024},
								},
								Env: []string{
									"IMAGE=ENV",
									"SOME=ENV",
									"http_proxy=http://proxy.com",
									"https_proxy=https://proxy.com",
									"no_proxy=http://noproxy.com",
								},
							}))
						})
					})

					Context("outputs are nested under outputs", func() {
						BeforeEach(func() {
							containerSpec.Inputs = []InputSource{}
							containerSpec.Outputs = OutputPaths{
								"some-output":              "/some/work-dir/output",
								"some-output-under-output": "/some/work-dir/output/other-output",
							}
						})

						It("creates the container with correct bind mounts", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							actualSpec := fakeGardenClient.CreateArgsForCall(0)
							Expect(actualSpec).To(Equal(garden.ContainerSpec{
								Handle:     "some-handle",
								RootFSPath: "some-image-url",
								Properties: garden.Properties{"user": "some-user"},
								BindMounts: []garden.BindMount{
									{
										SrcPath: "some/source",
										DstPath: "some/destination",
										Mode:    garden.BindMountModeRO,
									},
									{
										SrcPath: "/fake/scratch/volume",
										DstPath: "/scratch",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/work-dir/volume",
										DstPath: "/some/work-dir",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/output/volume",
										DstPath: "/some/work-dir/output",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/output/other-output/volume",
										DstPath: "/some/work-dir/output/other-output",
										Mode:    garden.BindMountModeRW,
									},
								},
								Limits: garden.Limits{
									CPU:    garden.CPULimits{LimitInShares: 1024},
									Memory: garden.MemoryLimits{LimitInBytes: 1024},
								},
								Env: []string{
									"IMAGE=ENV",
									"SOME=ENV",
									"http_proxy=http://proxy.com",
									"https_proxy=https://proxy.com",
									"no_proxy=http://noproxy.com",
								},
							}))
						})
					})

					Context("inputs are nested under outputs", func() {
						BeforeEach(func() {
							containerSpec.Inputs = []InputSource{
								fakeRemoteInputUnderOutput,
							}
							containerSpec.Outputs = OutputPaths{
								"some-output": "/some/work-dir/output",
							}
						})

						It("creates the container with correct bind mounts", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							actualSpec := fakeGardenClient.CreateArgsForCall(0)
							Expect(actualSpec).To(Equal(garden.ContainerSpec{
								Handle:     "some-handle",
								RootFSPath: "some-image-url",
								Properties: garden.Properties{"user": "some-user"},
								BindMounts: []garden.BindMount{
									{
										SrcPath: "some/source",
										DstPath: "some/destination",
										Mode:    garden.BindMountModeRO,
									},
									{
										SrcPath: "/fake/scratch/volume",
										DstPath: "/scratch",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/work-dir/volume",
										DstPath: "/some/work-dir",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/output/volume",
										DstPath: "/some/work-dir/output",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/output/input/container/volume",
										DstPath: "/some/work-dir/output/input",
										Mode:    garden.BindMountModeRW,
									},
								},
								Limits: garden.Limits{
									CPU:    garden.CPULimits{LimitInShares: 1024},
									Memory: garden.MemoryLimits{LimitInBytes: 1024},
								},
								Env: []string{
									"IMAGE=ENV",
									"SOME=ENV",
									"http_proxy=http://proxy.com",
									"https_proxy=https://proxy.com",
									"no_proxy=http://noproxy.com",
								},
							}))

						})
					})

					Context("input and output share the same destination path", func() {
						BeforeEach(func() {
							containerSpec.Inputs = []InputSource{
								fakeRemoteInput,
							}
							containerSpec.Outputs = OutputPaths{
								"some-output": "/some/work-dir/remote-input",
							}
						})

						It("creates the container with correct bind mounts", func() {
							Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

							actualSpec := fakeGardenClient.CreateArgsForCall(0)
							Expect(actualSpec).To(Equal(garden.ContainerSpec{
								Handle:     "some-handle",
								RootFSPath: "some-image-url",
								Properties: garden.Properties{"user": "some-user"},
								BindMounts: []garden.BindMount{
									{
										SrcPath: "some/source",
										DstPath: "some/destination",
										Mode:    garden.BindMountModeRO,
									},
									{
										SrcPath: "/fake/scratch/volume",
										DstPath: "/scratch",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/work-dir/volume",
										DstPath: "/some/work-dir",
										Mode:    garden.BindMountModeRW,
									},
									{
										SrcPath: "/fake/remote/input/container/volume",
										DstPath: "/some/work-dir/remote-input",
										Mode:    garden.BindMountModeRW,
									},
								},
								Limits: garden.Limits{
									CPU:    garden.CPULimits{LimitInShares: 1024},
									Memory: garden.MemoryLimits{LimitInBytes: 1024},
								},
								Env: []string{
									"IMAGE=ENV",
									"SOME=ENV",
									"http_proxy=http://proxy.com",
									"https_proxy=https://proxy.com",
									"no_proxy=http://noproxy.com",
								},
							}))
						})

					})
				})

				Context("when the certs volume does not exist on the worker", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
					})
					It("creates the container in garden, but does not bind mount any certs", func() {
						Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))
						actualSpec := fakeGardenClient.CreateArgsForCall(0)
						Expect(actualSpec.BindMounts).ToNot(ContainElement(
							garden.BindMount{
								SrcPath: "/the/certs/volume/path",
								DstPath: "/etc/ssl/certs",
								Mode:    garden.BindMountModeRO,
							},
						))
					})
				})

				It("creates each volume unprivileged", func() {
					Expect(volumeSpecs).To(Equal(map[string]VolumeSpec{
						"/scratch":                    {Strategy: baggageclaim.EmptyStrategy{}},
						"/some/work-dir":              {Strategy: baggageclaim.EmptyStrategy{}},
						"/some/work-dir/output":       {Strategy: baggageclaim.EmptyStrategy{}},
						"/some/work-dir/local-input":  {Strategy: fakeLocalVolume.COWStrategy()},
						"/some/work-dir/remote-input": {Strategy: baggageclaim.EmptyStrategy{}},
					}))
				})

				It("streams remote inputs into newly created container volumes", func() {
					Expect(fakeRemoteInputAS.StreamToCallCount()).To(Equal(1))
					_, _, ad := fakeRemoteInputAS.StreamToArgsForCall(0)

					err := ad.StreamIn(context.TODO(), ".", baggageclaim.GzipEncoding, bytes.NewBufferString("some-stream"))
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeRemoteInputContainerVolume.StreamInCallCount()).To(Equal(1))

					_, dst, encoding, from := fakeRemoteInputContainerVolume.StreamInArgsForCall(0)
					Expect(dst).To(Equal("."))
					Expect(encoding).To(Equal(baggageclaim.GzipEncoding))
					Expect(ioutil.ReadAll(from)).To(Equal([]byte("some-stream")))
				})

				It("marks container as created", func() {
					Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(1))
				})

				Context("when the fetched image was privileged", func() {
					BeforeEach(func() {
						fakeImage.FetchForContainerReturns(FetchedImage{
							Privileged: true,
							Metadata: ImageMetadata{
								Env: []string{"IMAGE=ENV"},
							},
							URL: "some-image-url",
						}, nil)
					})

					It("creates the container privileged", func() {
						Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

						actualSpec := fakeGardenClient.CreateArgsForCall(0)
						Expect(actualSpec.Privileged).To(BeTrue())
					})

					It("creates each volume privileged", func() {
						Expect(volumeSpecs).To(Equal(map[string]VolumeSpec{
							"/scratch":                    {Privileged: true, Strategy: baggageclaim.EmptyStrategy{}},
							"/some/work-dir":              {Privileged: true, Strategy: baggageclaim.EmptyStrategy{}},
							"/some/work-dir/output":       {Privileged: true, Strategy: baggageclaim.EmptyStrategy{}},
							"/some/work-dir/local-input":  {Privileged: true, Strategy: fakeLocalVolume.COWStrategy()},
							"/some/work-dir/remote-input": {Privileged: true, Strategy: baggageclaim.EmptyStrategy{}},
						}))
					})

				})

				Context("when an input has the path set to the workdir itself", func() {
					BeforeEach(func() {
						fakeLocalInput.DestinationPathReturns("/some/work-dir")
						delete(stubbedVolumes, "/some/work-dir/local-input")
						stubbedVolumes["/some/work-dir"] = fakeLocalCOWVolume
					})

					It("does not create or mount a work-dir, as we support this for backwards-compatibility", func() {
						Expect(fakeGardenClient.CreateCallCount()).To(Equal(1))

						actualSpec := fakeGardenClient.CreateArgsForCall(0)
						Expect(actualSpec.BindMounts).To(Equal([]garden.BindMount{
							{
								SrcPath: "some/source",
								DstPath: "some/destination",
								Mode:    garden.BindMountModeRO,
							},
							{
								SrcPath: "/fake/scratch/volume",
								DstPath: "/scratch",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/fake/local/cow/volume",
								DstPath: "/some/work-dir",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/fake/output/volume",
								DstPath: "/some/work-dir/output",
								Mode:    garden.BindMountModeRW,
							},
							{
								SrcPath: "/fake/remote/input/container/volume",
								DstPath: "/some/work-dir/remote-input",
								Mode:    garden.BindMountModeRW,
							},
						}))
					})
				})

				Context("when failing to create container in garden", func() {
					BeforeEach(func() {
						fakeGardenClient.CreateReturns(nil, disasterErr)
					})

					It("returns an error", func() {
						Expect(findOrCreateErr).To(Equal(disasterErr))
					})

					It("does not mark container as created", func() {
						Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(0))
					})

					It("marks the container as failed", func() {
						Expect(fakeCreatingContainer.FailedCallCount()).To(Equal(1))
					})
				})

				Context("when failing to create container in garden", func() {
					BeforeEach(func() {
						fakeGardenClient.CreateReturns(nil, disasterErr)
					})

					It("returns an error", func() {
						Expect(findOrCreateErr).To(Equal(disasterErr))
					})

					It("does not mark container as created", func() {
						Expect(fakeCreatingContainer.CreatedCallCount()).To(Equal(0))
					})
				})
			})

		})

		Context("when container exists in database in created state", func() {
			BeforeEach(func() {
				fakeDBWorker.FindContainerReturns(nil, fakeCreatedContainer, nil)
			})

			It("does not create a new db container", func() {
				Expect(fakeDBWorker.CreateContainerCallCount()).To(Equal(0))
			})

			Context("when container exists in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(fakeGardenContainer, nil)
				})

				It("returns container", func() {
					Expect(findOrCreateErr).ToNot(HaveOccurred())
					Expect(findOrCreateContainer).ToNot(BeNil())
				})
			})

			Context("when container does not exist in garden", func() {
				var containerNotFoundErr error

				BeforeEach(func() {
					containerNotFoundErr = garden.ContainerNotFoundError{fakeCreatedContainer.Handle()}
					fakeGardenClient.LookupReturns(nil, containerNotFoundErr)
				})

				It("returns an error", func() {
					Expect(findOrCreateErr).To(Equal(containerNotFoundErr))
				})
			})
		})

		Context("when container does not exist in database", func() {

			BeforeEach(func() {
				fakeDBWorker.FindContainerReturns(nil, nil, nil)
				fakeDBWorker.CreateContainerReturns(fakeCreatingContainer, nil)
			})

			It("attemps to create container in the db", func() {
				Expect(fakeDBWorker.CreateContainerCallCount()).To(Equal(1))
			})

			Context("having db container creation erroring", func() {
				Context("with ContainerOwnerDisappearedError", func() {
					BeforeEach(func() {
						fakeDBWorker.CreateContainerReturns(nil, db.ContainerOwnerDisappearedError{})
					})

					It("fails w/ ResourceConfigCheckSessionExpiredError", func() {
						Expect(findOrCreateErr).To(HaveOccurred())
						Expect(findOrCreateErr).To(Equal(ResourceConfigCheckSessionExpiredError))
					})
				})

				Context("with a non-specific error", func() {
					BeforeEach(func() {
						fakeDBWorker.CreateContainerReturns(nil, errors.New("err"))
					})

					It("fails with the same err", func() {
						Expect(findOrCreateErr).To(HaveOccurred())
						Expect(findOrCreateErr).To(Equal(errors.New("err")))
					})
				})
			})

			Context("having db container creation succeeding", func() {
				It("creates a creating container in database", func() {
					owner, metadata := fakeDBWorker.CreateContainerArgsForCall(0)
					Expect(owner).To(Equal(fakeContainerOwner))
					Expect(metadata).To(Equal(containerMetadata))
				})
			})

		})
	})
})

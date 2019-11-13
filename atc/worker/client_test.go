package worker_test

import (
	"context"
	"errors"
	"fmt"
	"path"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var (
		logger          *lagertest.TestLogger
		fakePool        *workerfakes.FakePool
		fakeProvider    *workerfakes.FakeWorkerProvider
		client          worker.Client
		fakeLock        *lockfakes.FakeLock
		fakeLockFactory *lockfakes.FakeLockFactory
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakePool = new(workerfakes.FakePool)
		fakeProvider = new(workerfakes.FakeWorkerProvider)

		client = worker.NewClient(fakePool, fakeProvider)
	})

	Describe("FindContainer", func() {
		var (
			foundContainer worker.Container
			found          bool
			findErr        error
		)

		JustBeforeEach(func() {
			foundContainer, found, findErr = client.FindContainer(
				logger,
				4567,
				"some-handle",
			)
		})

		Context("when looking up the worker errors", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForContainerReturns(nil, false, errors.New("nope"))
			})

			It("errors", func() {
				Expect(findErr).To(HaveOccurred())
			})
		})

		Context("when worker is not found", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForContainerReturns(nil, false, nil)
			})

			It("returns not found", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when a worker is found with the container", func() {
			var fakeWorker *workerfakes.FakeWorker
			var fakeContainer *workerfakes.FakeContainer

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.FindWorkerForContainerReturns(fakeWorker, true, nil)

				fakeContainer = new(workerfakes.FakeContainer)
				fakeWorker.FindContainerByHandleReturns(fakeContainer, true, nil)
			})

			It("succeeds", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).NotTo(HaveOccurred())
			})

			It("returns the created container", func() {
				Expect(foundContainer).To(Equal(fakeContainer))
			})
		})
	})

	Describe("FindVolume", func() {
		var (
			foundVolume worker.Volume
			found       bool
			findErr     error
		)

		JustBeforeEach(func() {
			foundVolume, found, findErr = client.FindVolume(
				logger,
				4567,
				"some-handle",
			)
		})

		Context("when looking up the worker errors", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForVolumeReturns(nil, false, errors.New("nope"))
			})

			It("errors", func() {
				Expect(findErr).To(HaveOccurred())
			})
		})

		Context("when worker is not found", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForVolumeReturns(nil, false, nil)
			})

			It("returns not found", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when a worker is found with the volume", func() {
			var fakeWorker *workerfakes.FakeWorker
			var fakeVolume *workerfakes.FakeVolume

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.FindWorkerForVolumeReturns(fakeWorker, true, nil)

				fakeVolume = new(workerfakes.FakeVolume)
				fakeWorker.LookupVolumeReturns(fakeVolume, true, nil)
			})

			It("succeeds", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).NotTo(HaveOccurred())
			})

			It("returns the volume", func() {
				Expect(foundVolume).To(Equal(fakeVolume))
			})
		})
	})

	Describe("CreateVolume", func() {
		var (
			fakeWorker *workerfakes.FakeWorker
			volumeSpec worker.VolumeSpec
			workerSpec worker.WorkerSpec
			volumeType db.VolumeType
			err        error
		)

		BeforeEach(func() {
			volumeSpec = worker.VolumeSpec{
				Strategy: baggageclaim.EmptyStrategy{},
			}

			workerSpec = worker.WorkerSpec{
				TeamID: 1,
			}

			volumeType = db.VolumeTypeArtifact
		})

		JustBeforeEach(func() {
			_, err = client.CreateVolume(logger, volumeSpec, workerSpec, volumeType)
		})

		Context("when no workers can be found", func() {
			BeforeEach(func() {
				fakePool.FindOrChooseWorkerReturns(nil, errors.New("nope"))
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the worker can be found", func() {
			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakePool.FindOrChooseWorkerReturns(fakeWorker, nil)
			})

			It("creates the volume on the worker", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(1))
				l, spec, id, t := fakeWorker.CreateVolumeArgsForCall(0)
				Expect(l).To(Equal(logger))
				Expect(spec).To(Equal(volumeSpec))
				Expect(id).To(Equal(1))
				Expect(t).To(Equal(volumeType))
			})
		})
	})

	Describe("RunGetStep", func() {

		var (
			ctx                   context.Context
			owner                 db.ContainerOwner
			containerSpec         worker.ContainerSpec
			workerSpec            worker.WorkerSpec
			metadata              db.ContainerMetadata
			imageSpec             worker.ImageFetcherSpec
			fakeChosenWorker      *workerfakes.FakeWorker
			fakeStrategy          *workerfakes.FakeContainerPlacementStrategy
			fakeDelegate          *workerfakes.FakeImageFetchingDelegate
			fakeResourceTypes     atc.VersionedResourceTypes
			fakeContainer         *workerfakes.FakeContainer
			fakeProcessSpec       runtime.ProcessSpec
			fakeResource          *resourcefakes.FakeResource
			fakeUsedResourceCache *dbfakes.FakeUsedResourceCache

			err error

			disasterErr error

			result worker.GetResult
		)

		BeforeEach(func() {
			ctx, _ = context.WithCancel(context.Background())
			owner = new(dbfakes.FakeContainerOwner)
			containerSpec = worker.ContainerSpec{}
			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
			workerSpec = worker.WorkerSpec{}
			fakeChosenWorker = new(workerfakes.FakeWorker)
			fakeDelegate = new(workerfakes.FakeImageFetchingDelegate)
			fakeResourceTypes = atc.VersionedResourceTypes{}
			imageSpec = worker.ImageFetcherSpec{
				Delegate:      fakeDelegate,
				ResourceTypes: fakeResourceTypes,
			}

			fakeResource = new(resourcefakes.FakeResource)
			fakeContainer = new(workerfakes.FakeContainer)
			disasterErr = errors.New("oh no")
			stdout := new(gbytes.Buffer)
			stderr := new(gbytes.Buffer)
			fakeProcessSpec = runtime.ProcessSpec{
				Path:         "/opt/resource/out",
				StdoutWriter: stdout,
				StderrWriter: stderr,
			}
			fakeUsedResourceCache = new(dbfakes.FakeUsedResourceCache)

			fakeChosenWorker = new(workerfakes.FakeWorker)
			fakeChosenWorker.NameReturns("some-worker")
			fakeChosenWorker.SatisfiesReturns(true)
			fakeChosenWorker.FindOrCreateContainerReturns(fakeContainer, nil)
			fakePool.FindOrChooseWorkerForContainerReturns(fakeChosenWorker, nil)

		})

		JustBeforeEach(func() {
			result, err = client.RunGetStep(
				ctx,
				logger,
				owner,
				containerSpec,
				workerSpec,
				fakeStrategy,
				metadata,
				imageSpec,
				fakeProcessSpec,
				fakeUsedResourceCache,
				fakeResource,
			)
		})

		It("finds/chooses a worker", func() {
			Expect(err).ToNot(HaveOccurred())

			Expect(fakePool.FindOrChooseWorkerForContainerCallCount()).To(Equal(1))

			_, _, actualOwner, actualContainerSpec, actualWorkerSpec, actualStrategy := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
			Expect(actualOwner).To(Equal(owner))
			Expect(actualContainerSpec).To(Equal(containerSpec))
			Expect(actualWorkerSpec).To(Equal(workerSpec))
			Expect(actualStrategy).To(Equal(fakeStrategy))
		})

		Context("worker is chosen", func() {
			BeforeEach(func() {
				fakePool.FindOrChooseWorkerReturns(fakeChosenWorker, nil)
			})

			It("calls Fetch on the worker", func() {
				Expect(fakeChosenWorker.FetchCallCount()).To(Equal(1))
				_, _, actualMetadata, actualChosenWorker, actualContainerSpec, actualProcessSpec, actualResource, actualOwner, actualImageFetcherSpec, actualResourceCache, actualLockName := fakeChosenWorker.FetchArgsForCall(0)

				Expect(actualMetadata).To(Equal(metadata))
				Expect(actualChosenWorker).To(Equal(fakeChosenWorker))
				Expect(actualContainerSpec).To(Equal(containerSpec))
				Expect(actualProcessSpec).To(Equal(fakeProcessSpec))
				Expect(actualResource).To(Equal(fakeResource))
				Expect(actualOwner).To(Equal(owner))
				Expect(actualImageFetcherSpec).To(Equal(imageSpec))
				Expect(actualResourceCache).To(Equal(fakeUsedResourceCache))
				// Computed SHA
				Expect(actualLockName).To(Equal("18c3de3f8ea112ba52e01f279b6cc62335b4bec2f359b9be7636a5ad7bf98f8c"))
			})
		})

		Context("Worker selection returns an error", func() {
			BeforeEach(func() {
				fakePool.FindOrChooseWorkerForContainerReturns(nil, disasterErr)
			})

			It("Returns the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disasterErr))

				Expect(result).To(Equal(worker.GetResult{}))
			})
		})

		Context("Worker.Fetch returns an error", func() {
			BeforeEach(func() {
				fakePool.FindOrChooseWorkerReturns(fakeChosenWorker, nil)
				fakeChosenWorker.FetchReturns(worker.GetResult{}, nil, disasterErr)
			})

			It("Returns the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disasterErr))

				Expect(result).To(Equal(worker.GetResult{}))
			})
		})

		Context("worker.Fetch is successful", func() {
			var (
				fakeVolume    *workerfakes.FakeVolume
				someGetResult worker.GetResult
			)
			BeforeEach(func() {
				someGetResult = worker.GetResult{
					Status: 0,
					VersionResult: runtime.VersionResult{
						atc.Version{"some-version": "some-value"},
						[]atc.MetadataField{{"foo", "bar"}},
					},
				}
				fakeVolume = new(workerfakes.FakeVolume)
				fakeChosenWorker.FetchReturns(someGetResult, fakeVolume, nil)
			})

			It("returns getResult from worker->Fetch", func() {
				Expect(result).To(Equal(someGetResult))
				Expect(err).ToNot(HaveOccurred())
			})

		})
	})

	Describe("RunTaskStep", func() {
		var (
			status       int
			volumeMounts []worker.VolumeMount
			inputSources []worker.InputSource
			err          error

			fakeWorker           *workerfakes.FakeWorker
			fakeContainerOwner   db.ContainerOwner
			fakeWorkerSpec       worker.WorkerSpec
			fakeContainerSpec    worker.ContainerSpec
			fakeStrategy         *workerfakes.FakeContainerPlacementStrategy
			fakeMetadata         db.ContainerMetadata
			fakeDelegate         *execfakes.FakeTaskDelegate
			fakeImageFetcherSpec worker.ImageFetcherSpec
			fakeTaskProcessSpec  runtime.ProcessSpec
			fakeContainer        *workerfakes.FakeContainer
			ctx                  context.Context
			cancel               func()
		)

		BeforeEach(func() {
			cpu := uint64(1024)
			memory := uint64(1024)

			buildId := 1234
			planId := atc.PlanID(42)
			teamId := 123
			fakeDelegate = new(execfakes.FakeTaskDelegate)
			fakeContainerOwner = db.NewBuildStepContainerOwner(
				buildId,
				planId,
				teamId,
			)
			fakeWorkerSpec = worker.WorkerSpec{}
			fakeContainerSpec = worker.ContainerSpec{
				Platform: "some-platform",
				Tags:     []string{"step", "tags"},
				TeamID:   123,
				ImageSpec: worker.ImageSpec{
					ImageResource: &worker.ImageResource{
						Type:    "docker",
						Source:  atc.Source{"some": "secret-source-param"},
						Params:  atc.Params{"some": "params"},
						Version: atc.Version{"some": "version"},
					},
					Privileged: false,
				},
				Limits: worker.ContainerLimits{
					CPU:    &cpu,
					Memory: &memory,
				},
				Dir:          "some-artifact-root",
				Env:          []string{"SECURE=secret-task-param"},
				InputFooBars: nil,
				Inputs:       inputSources,
				Outputs:      worker.OutputPaths{},
			}
			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
			fakeMetadata = db.ContainerMetadata{
				WorkingDirectory: "some-artifact-root",
				Type:             db.ContainerTypeTask,
				StepName:         "some-step",
			}
			fakeImageFetcherSpec = worker.ImageFetcherSpec{
				Delegate:      fakeDelegate,
				ResourceTypes: atc.VersionedResourceTypes{},
			}
			fakeTaskProcessSpec = runtime.ProcessSpec{
				Path: "/some/path",
				Args: []string{"some", "args"},
				Dir:  "/some/dir",
			}
			fakeContainer = new(workerfakes.FakeContainer)
			fakeContainer.PropertiesReturns(garden.Properties{"concourse:exit-status": "0"}, nil)

			fakeWorker = new(workerfakes.FakeWorker)
			fakeWorker.NameReturns("some-worker")
			fakeWorker.SatisfiesReturns(true)
			fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)

			fakeWorker.IncreaseActiveTasksStub = func() error {
				fakeWorker.ActiveTasksReturns(1, nil)
				return nil
			}

			fakeWorker.DecreaseActiveTasksStub = func() error {
				fakeWorker.ActiveTasksReturns(0, nil)
				return nil
			}

			fakeLockFactory = new(lockfakes.FakeLockFactory)
			fakeLock = new(lockfakes.FakeLock)
			fakeLockFactory.AcquireReturns(fakeLock, true, nil)

			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)
			ctx, cancel = context.WithCancel(context.Background())
		})

		JustBeforeEach(func() {
			taskResult := client.RunTaskStep(
				ctx,
				logger,
				fakeContainerOwner,
				fakeContainerSpec,
				fakeWorkerSpec,
				fakeStrategy,
				fakeMetadata,
				fakeImageFetcherSpec,
				fakeTaskProcessSpec,
				fakeLockFactory,
			)
			status = taskResult.Status
			volumeMounts = taskResult.VolumeMounts
			err = taskResult.Err
		})

		Context("choosing a worker", func() {
			BeforeEach(func() {
				// later fakes are uninitialized
				fakeContainer.PropertiesReturns(garden.Properties{"concourse:exit-status": "3"}, nil)
			})

			It("chooses a worker", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(fakePool.FindOrChooseWorkerForContainerCallCount()).To(Equal(1))
			})

			Context("when 'limit-active-tasks' strategy is chosen", func() {
				BeforeEach(func() {
					fakeStrategy.ModifiesActiveTasksReturns(true)
				})
				Context("when a worker is found", func() {
					BeforeEach(func() {
						fakeWorker.NameReturns("some-worker")
						fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)

						fakeContainer := new(workerfakes.FakeContainer)
						fakeWorker.FindOrCreateContainerReturns(fakeContainer, nil)
						fakeContainer.PropertiesReturns(garden.Properties{"concourse:exit-status": "0"}, nil)
					})
					It("increase the active tasks on the worker", func() {
						Expect(fakeWorker.IncreaseActiveTasksCallCount()).To(Equal(1))
					})

					Context("when the container is already present on the worker", func() {
						BeforeEach(func() {
							fakePool.ContainerInWorkerReturns(true, nil)
						})
						It("does not increase the active tasks on the worker", func() {
							Expect(fakeWorker.IncreaseActiveTasksCallCount()).To(Equal(0))
						})
					})
				})

				Context("when the task is aborted waiting for an available worker", func() {
					BeforeEach(func() {
						cancel()
					})
					It("exits releasing the lock", func() {
						Expect(err).To(Equal(context.Canceled))
						Expect(status).To(Equal(-1))
						Expect(fakeLock.ReleaseCallCount()).To(Equal(fakeLockFactory.AcquireCallCount()))
					})
				})

				Context("when a container in worker returns an error", func() {
					BeforeEach(func() {
						fakePool.ContainerInWorkerReturns(false, errors.New("nope"))
					})
					It("release the task-step lock every time it acquires it", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(fakeLockFactory.AcquireCallCount()))
					})
				})
			})

			Context("when finding or choosing the worker fails", func() {
				workerDisaster := errors.New("worker selection failed")

				BeforeEach(func() {
					fakePool.FindOrChooseWorkerForContainerReturns(nil, workerDisaster)
				})

				It("returns the error", func() {
					Expect(err).To(Equal(workerDisaster))
				})
			})

		})

		It("finds or creates a container", func() {
			Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
			_, cancel, delegate, owner, createdMetadata, containerSpec, _ := fakeWorker.FindOrCreateContainerArgsForCall(0)
			Expect(containerSpec.Inputs).To(Equal(fakeContainerSpec.Inputs))
			Expect(containerSpec).To(Equal(fakeContainerSpec))
			Expect(cancel).ToNot(BeNil())
			Expect(owner).To(Equal(fakeContainerOwner))
			Expect(delegate).To(Equal(fakeDelegate))
			Expect(createdMetadata).To(Equal(db.ContainerMetadata{
				WorkingDirectory: "some-artifact-root",
				Type:             db.ContainerTypeTask,
				StepName:         "some-step",
			}))

		})

		Context("found a container that has already exited", func() {
			BeforeEach(func() {
				fakeContainer.PropertiesReturns(garden.Properties{"concourse:exit-status": "8"}, nil)
			})

			It("does not attach to any process", func() {
				Expect(fakeContainer.AttachCallCount()).To(BeZero())
			})

			It("returns result of container process", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(8))
			})

			Context("when 'limit-active-tasks' strategy is chosen", func() {
				BeforeEach(func() {
					fakeStrategy.ModifiesActiveTasksReturns(true)
				})

				It("decrements the active tasks counter on the worker", func() {
					Expect(fakeWorker.ActiveTasks()).To(Equal(0))
				})
			})

			Context("when volumes are configured and present on the container", func() {
				var (
					fakeMountPath1 = "some-artifact-root/some-output-configured-path/"
					fakeMountPath2 = "some-artifact-root/some-other-output/"
					fakeMountPath3 = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

					fakeVolume1 *workerfakes.FakeVolume
					fakeVolume2 *workerfakes.FakeVolume
					fakeVolume3 *workerfakes.FakeVolume
				)

				BeforeEach(func() {
					fakeVolume1 = new(workerfakes.FakeVolume)
					fakeVolume1.HandleReturns("some-handle-1")
					fakeVolume2 = new(workerfakes.FakeVolume)
					fakeVolume2.HandleReturns("some-handle-2")
					fakeVolume3 = new(workerfakes.FakeVolume)
					fakeVolume3.HandleReturns("some-handle-3")

					fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
						worker.VolumeMount{
							Volume:    fakeVolume1,
							MountPath: fakeMountPath1,
						},
						worker.VolumeMount{
							Volume:    fakeVolume2,
							MountPath: fakeMountPath2,
						},
						worker.VolumeMount{
							Volume:    fakeVolume3,
							MountPath: fakeMountPath3,
						},
					})
				})

				It("returns all the volume mounts", func() {
					Expect(volumeMounts).To(ConsistOf(
						worker.VolumeMount{
							Volume:    fakeVolume1,
							MountPath: fakeMountPath1,
						},
						worker.VolumeMount{
							Volume:    fakeVolume2,
							MountPath: fakeMountPath2,
						},
						worker.VolumeMount{
							Volume:    fakeVolume3,
							MountPath: fakeMountPath3,
						},
					))
				})
			})
		})

		Context("container has not already exited", func() {
			var (
				fakeProcess         *gardenfakes.FakeProcess
				fakeProcessExitCode int

				fakeMountPath1 = "some-artifact-root/some-output-configured-path/"
				fakeMountPath2 = "some-artifact-root/some-other-output/"
				fakeMountPath3 = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

				fakeVolume1 *workerfakes.FakeVolume
				fakeVolume2 *workerfakes.FakeVolume
				fakeVolume3 *workerfakes.FakeVolume

				stdoutBuf *gbytes.Buffer
				stderrBuf *gbytes.Buffer
			)

			BeforeEach(func() {
				fakeProcess = new(gardenfakes.FakeProcess)
				fakeContainer.PropertiesReturns(garden.Properties{}, nil)

				// for testing volume mounts being returned
				fakeVolume1 = new(workerfakes.FakeVolume)
				fakeVolume1.HandleReturns("some-handle-1")
				fakeVolume2 = new(workerfakes.FakeVolume)
				fakeVolume2.HandleReturns("some-handle-2")
				fakeVolume3 = new(workerfakes.FakeVolume)
				fakeVolume3.HandleReturns("some-handle-3")

				fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
					{
						Volume:    fakeVolume1,
						MountPath: fakeMountPath1,
					},
					{
						Volume:    fakeVolume2,
						MountPath: fakeMountPath2,
					},
					{
						Volume:    fakeVolume3,
						MountPath: fakeMountPath3,
					},
				})
			})

			Context("found container that is already running", func() {
				BeforeEach(func() {
					fakeContainer.AttachReturns(fakeProcess, nil)

					stdoutBuf = new(gbytes.Buffer)
					stderrBuf = new(gbytes.Buffer)
					fakeTaskProcessSpec = runtime.ProcessSpec{
						StdoutWriter: stdoutBuf,
						StderrWriter: stderrBuf,
					}
				})

				It("does not create a new container", func() {
					Expect(fakeContainer.RunCallCount()).To(BeZero())
				})

				It("attaches to the running process", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(fakeContainer.AttachCallCount()).To(Equal(1))
					Expect(fakeContainer.RunCallCount()).To(Equal(0))
					_, _, actualProcessIO := fakeContainer.AttachArgsForCall(0)
					Expect(actualProcessIO.Stdout).To(Equal(stdoutBuf))
					Expect(actualProcessIO.Stderr).To(Equal(stderrBuf))
				})

				Context("when the process is interrupted", func() {
					var stopped chan struct{}
					BeforeEach(func() {
						stopped = make(chan struct{})

						fakeProcess.WaitStub = func() (int, error) {
							defer GinkgoRecover()

							<-stopped
							return 128 + 15, nil
						}

						fakeContainer.StopStub = func(bool) error {
							close(stopped)
							return nil
						}

						cancel()
					})

					It("stops the container", func() {
						Expect(fakeContainer.StopCallCount()).To(Equal(1))
						Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
						Expect(err).To(Equal(context.Canceled))
					})

					Context("when container.stop returns an error", func() {
						var disaster error

						BeforeEach(func() {
							disaster = errors.New("gotta get away")

							fakeContainer.StopStub = func(bool) error {
								close(stopped)
								return disaster
							}
						})

						It("doesn't return the error", func() {
							Expect(err).To(Equal(context.Canceled))
						})
					})

					Context("when 'limit-active-tasks' strategy is chosen", func() {
						BeforeEach(func() {
							fakeStrategy.ModifiesActiveTasksReturns(true)
						})

						It("decrements the active tasks counter on the worker", func() {
							Expect(fakeWorker.ActiveTasks()).To(Equal(0))
						})
					})
				})

				Context("when the process exits successfully", func() {
					BeforeEach(func() {
						fakeProcessExitCode = 0
						fakeProcess.WaitReturns(fakeProcessExitCode, nil)
					})
					It("returns a successful result", func() {
						Expect(status).To(BeZero())
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns all the volume mounts", func() {
						Expect(volumeMounts).To(ConsistOf(
							worker.VolumeMount{
								Volume:    fakeVolume1,
								MountPath: fakeMountPath1,
							},
							worker.VolumeMount{
								Volume:    fakeVolume2,
								MountPath: fakeMountPath2,
							},
							worker.VolumeMount{
								Volume:    fakeVolume3,
								MountPath: fakeMountPath3,
							},
						))
					})

					Context("when 'limit-active-tasks' strategy is chosen", func() {
						BeforeEach(func() {
							fakeStrategy.ModifiesActiveTasksReturns(true)
						})

						It("decrements the active tasks counter on the worker", func() {
							Expect(fakeWorker.ActiveTasks()).To(Equal(0))
						})
					})
				})

				Context("when the process exits with an error", func() {
					disaster := errors.New("process failed")
					BeforeEach(func() {
						fakeProcessExitCode = 128 + 15
						fakeProcess.WaitReturns(fakeProcessExitCode, disaster)
					})
					It("returns an unsuccessful result", func() {
						Expect(status).To(Equal(fakeProcessExitCode))
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(disaster))
					})

					It("returns no volume mounts", func() {
						Expect(volumeMounts).To(BeEmpty())
					})

					Context("when 'limit-active-tasks' strategy is chosen", func() {
						BeforeEach(func() {
							fakeStrategy.ModifiesActiveTasksReturns(true)
						})

						It("decrements the active tasks counter on the worker", func() {
							Expect(fakeWorker.ActiveTasks()).To(Equal(0))
						})
					})
				})
			})

			Context("created a new container", func() {
				BeforeEach(func() {
					fakeContainer.AttachReturns(nil, errors.New("container not running"))
					fakeContainer.RunReturns(fakeProcess, nil)

					stdoutBuf = new(gbytes.Buffer)
					stderrBuf = new(gbytes.Buffer)
					fakeTaskProcessSpec = runtime.ProcessSpec{
						StdoutWriter: stdoutBuf,
						StderrWriter: stderrBuf,
					}
				})

				It("runs a new process in the container", func() {
					Eventually(fakeContainer.RunCallCount()).Should(Equal(1))

					_, gardenProcessSpec, actualProcessIO := fakeContainer.RunArgsForCall(0)
					Expect(gardenProcessSpec.ID).To(Equal("task"))
					Expect(gardenProcessSpec.Path).To(Equal(fakeTaskProcessSpec.Path))
					Expect(gardenProcessSpec.Args).To(ConsistOf(fakeTaskProcessSpec.Args))
					Expect(gardenProcessSpec.Dir).To(Equal(path.Join(fakeMetadata.WorkingDirectory, fakeTaskProcessSpec.Dir)))
					Expect(gardenProcessSpec.TTY).To(Equal(&garden.TTYSpec{WindowSize: &garden.WindowSize{Columns: 500, Rows: 500}}))
					Expect(actualProcessIO.Stdout).To(Equal(stdoutBuf))
					Expect(actualProcessIO.Stderr).To(Equal(stderrBuf))
				})

				Context("when the process is interrupted", func() {
					var stopped chan struct{}
					BeforeEach(func() {
						stopped = make(chan struct{})

						fakeProcess.WaitStub = func() (int, error) {
							defer GinkgoRecover()

							<-stopped
							return 128 + 15, nil // wat?
						}

						fakeContainer.StopStub = func(bool) error {
							close(stopped)
							return nil
						}

						cancel()
					})

					It("stops the container", func() {
						Expect(fakeContainer.StopCallCount()).To(Equal(1))
						Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
						Expect(err).To(Equal(context.Canceled))
					})

					Context("when container.stop returns an error", func() {
						var disaster error

						BeforeEach(func() {
							disaster = errors.New("gotta get away")

							fakeContainer.StopStub = func(bool) error {
								close(stopped)
								return disaster
							}
						})

						It("doesn't return the error", func() {
							Expect(err).To(Equal(context.Canceled))
						})
					})
				})

				Context("when the process exits successfully", func() {
					It("returns a successful result", func() {
						Expect(status).To(BeZero())
						Expect(err).ToNot(HaveOccurred())
					})

					It("saves the exit status property", func() {
						Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Expect(name).To(Equal("concourse:exit-status"))
						Expect(value).To(Equal("0"))
					})

					Context("when saving the exit status succeeds", func() {
						BeforeEach(func() {
							fakeContainer.SetPropertyReturns(nil)
						})

						It("returns successfully", func() {
							Expect(err).ToNot(HaveOccurred())
						})
					})

					Context("when saving the exit status fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeContainer.SetPropertyStub = func(name string, value string) error {
								defer GinkgoRecover()

								if name == "concourse:exit-status" {
									return disaster
								}

								return nil
							}
						})

						It("returns the error", func() {
							Expect(err).To(Equal(disaster))
						})
					})

					Context("when volumes are configured and present on the container", func() {
						var (
							fakeMountPath1 = "some-artifact-root/some-output-configured-path/"
							fakeMountPath2 = "some-artifact-root/some-other-output/"
							fakeMountPath3 = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

							fakeVolume1 *workerfakes.FakeVolume
							fakeVolume2 *workerfakes.FakeVolume
							fakeVolume3 *workerfakes.FakeVolume
						)

						BeforeEach(func() {
							fakeVolume1 = new(workerfakes.FakeVolume)
							fakeVolume1.HandleReturns("some-handle-1")
							fakeVolume2 = new(workerfakes.FakeVolume)
							fakeVolume2.HandleReturns("some-handle-2")
							fakeVolume3 = new(workerfakes.FakeVolume)
							fakeVolume3.HandleReturns("some-handle-3")

							fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
								worker.VolumeMount{
									Volume:    fakeVolume1,
									MountPath: fakeMountPath1,
								},
								worker.VolumeMount{
									Volume:    fakeVolume2,
									MountPath: fakeMountPath2,
								},
								worker.VolumeMount{
									Volume:    fakeVolume3,
									MountPath: fakeMountPath3,
								},
							})
						})

						It("returns all the volume mounts", func() {
							Expect(volumeMounts).To(ConsistOf(
								worker.VolumeMount{
									Volume:    fakeVolume1,
									MountPath: fakeMountPath1,
								},
								worker.VolumeMount{
									Volume:    fakeVolume2,
									MountPath: fakeMountPath2,
								},
								worker.VolumeMount{
									Volume:    fakeVolume3,
									MountPath: fakeMountPath3,
								},
							))
						})

					})

					Context("when 'limit-active-tasks' strategy is chosen", func() {
						BeforeEach(func() {
							fakeStrategy.ModifiesActiveTasksReturns(true)
						})

						It("decrements the active tasks counter on the worker", func() {
							Expect(fakeWorker.ActiveTasks()).To(Equal(0))
						})
					})
				})

				Context("when the process exits on failure", func() {
					BeforeEach(func() {
						fakeProcessExitCode = 128 + 15
						fakeProcess.WaitReturns(fakeProcessExitCode, nil)
					})
					It("returns an unsuccessful result", func() {
						Expect(status).To(Equal(fakeProcessExitCode))
						Expect(err).ToNot(HaveOccurred())
					})

					It("saves the exit status property", func() {
						Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Expect(name).To(Equal("concourse:exit-status"))
						Expect(value).To(Equal(fmt.Sprint(fakeProcessExitCode)))
					})

					Context("when saving the exit status succeeds", func() {
						BeforeEach(func() {
							fakeContainer.PropertiesReturns(garden.Properties{"concourse:exit-status": "0"}, nil)
						})

						It("returns successfully", func() {
							Expect(err).ToNot(HaveOccurred())
						})
					})

					Context("when saving the exit status fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeContainer.SetPropertyStub = func(name string, value string) error {
								defer GinkgoRecover()

								if name == "concourse:exit-status" {
									return disaster
								}

								return nil
							}
						})

						It("returns the error", func() {
							Expect(err).To(Equal(disaster))
						})
					})

					It("returns all the volume mounts", func() {
						Expect(volumeMounts).To(ConsistOf(
							worker.VolumeMount{
								Volume:    fakeVolume1,
								MountPath: fakeMountPath1,
							},
							worker.VolumeMount{
								Volume:    fakeVolume2,
								MountPath: fakeMountPath2,
							},
							worker.VolumeMount{
								Volume:    fakeVolume3,
								MountPath: fakeMountPath3,
							},
						))
					})

					Context("when 'limit-active-tasks' strategy is chosen", func() {
						BeforeEach(func() {
							fakeStrategy.ModifiesActiveTasksReturns(true)
						})

						It("decrements the active tasks counter on the worker", func() {
							Expect(fakeWorker.ActiveTasks()).To(Equal(0))
						})
					})
				})

				Context("when running the container fails with an error", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeContainer.RunReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(err).To(Equal(disaster))
					})

					Context("when 'limit-active-tasks' strategy is chosen", func() {
						BeforeEach(func() {
							fakeStrategy.ModifiesActiveTasksReturns(true)
						})

						It("decrements the active tasks counter on the worker", func() {
							Expect(fakeWorker.ActiveTasks()).To(Equal(0))
						})
					})
				})
			})
		})
	})

	Describe("RunPutStep", func() {

		var (
			ctx               context.Context
			owner             db.ContainerOwner
			containerSpec     worker.ContainerSpec
			workerSpec        worker.WorkerSpec
			metadata          db.ContainerMetadata
			imageSpec         worker.ImageFetcherSpec
			fakeChosenWorker  *workerfakes.FakeWorker
			fakeStrategy      *workerfakes.FakeContainerPlacementStrategy
			fakeDelegate      *workerfakes.FakeImageFetchingDelegate
			fakeResourceTypes atc.VersionedResourceTypes
			fakeContainer     *workerfakes.FakeContainer
			fakeProcessSpec   runtime.ProcessSpec
			fakeResource      *resourcefakes.FakeResource

			versionResult runtime.VersionResult
			status        int
			err           error

			disasterErr error
		)

		BeforeEach(func() {
			ctx = context.Background()
			owner = new(dbfakes.FakeContainerOwner)
			containerSpec = worker.ContainerSpec{}
			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
			workerSpec = worker.WorkerSpec{}
			fakeChosenWorker = new(workerfakes.FakeWorker)
			fakeDelegate = new(workerfakes.FakeImageFetchingDelegate)
			fakeResourceTypes = atc.VersionedResourceTypes{}
			imageSpec = worker.ImageFetcherSpec{
				Delegate:      fakeDelegate,
				ResourceTypes: fakeResourceTypes,
			}

			fakeContainer = new(workerfakes.FakeContainer)
			disasterErr = errors.New("oh no")
			stdout := new(gbytes.Buffer)
			stderr := new(gbytes.Buffer)
			fakeProcessSpec = runtime.ProcessSpec{
				Path:         "/opt/resource/out",
				StdoutWriter: stdout,
				StderrWriter: stderr,
			}
			fakeResource = new(resourcefakes.FakeResource)

			fakeChosenWorker = new(workerfakes.FakeWorker)
			fakeChosenWorker.NameReturns("some-worker")
			fakeChosenWorker.SatisfiesReturns(true)
			fakeChosenWorker.FindOrCreateContainerReturns(fakeContainer, nil)
			fakePool.FindOrChooseWorkerForContainerReturns(fakeChosenWorker, nil)

		})

		JustBeforeEach(func() {
			result := client.RunPutStep(
				ctx,
				logger,
				owner,
				containerSpec,
				workerSpec,
				fakeStrategy,
				metadata,
				imageSpec,
				fakeProcessSpec,
				fakeResource,
			)
			versionResult = result.VersionResult
			err = result.Err
			status = result.Status
		})

		It("finds/chooses a worker", func() {
			Expect(fakePool.FindOrChooseWorkerForContainerCallCount()).To(Equal(1))

			_, _, actualOwner, actualContainerSpec, actualWorkerSpec, strategy := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
			Expect(actualOwner).To(Equal(owner))
			Expect(actualContainerSpec).To(Equal(containerSpec))
			Expect(actualWorkerSpec).To(Equal(workerSpec))
			Expect(strategy).To(Equal(fakeStrategy))
		})

		Context("worker is chosen", func() {
			BeforeEach(func() {
				fakePool.FindOrChooseWorkerReturns(fakeChosenWorker, nil)
			})
			It("finds or creates a put container on that worker", func() {
				Expect(fakeChosenWorker.FindOrCreateContainerCallCount()).To(Equal(1))
				_, _, actualDelegate, actualOwner, actualMetadata, actualContainerSpec, actualResourceTypes := fakeChosenWorker.FindOrCreateContainerArgsForCall(0)

				Expect(actualContainerSpec).To(Equal(containerSpec))
				Expect(actualDelegate).To(Equal(fakeDelegate))
				Expect(actualOwner).To(Equal(owner))
				Expect(actualMetadata).To(Equal(metadata))
				Expect(actualResourceTypes).To(Equal(fakeResourceTypes))
			})
		})

		Context("worker selection returns an error", func() {
			BeforeEach(func() {
				fakePool.FindOrChooseWorkerForContainerReturns(nil, disasterErr)
			})

			It("returns the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disasterErr))
				Expect(versionResult).To(Equal(runtime.VersionResult{}))
			})
		})

		Context("found a container that has run resource.Put and exited", func() {
			BeforeEach(func() {
				fakeChosenWorker.FindOrCreateContainerReturns(fakeContainer, nil)
				fakeContainer.PropertyStub = func(prop string) (result string, err error) {
					if prop == "concourse:exit-status" {
						return "8", nil
					}
					return "", errors.New("unhandled property")
				}
			})

			It("does not invoke resource.Put", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(0))
			})

			It("returns result of container process", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(8))
			})
		})

		Context("calling resource.Put", func() {
			BeforeEach(func() {
				fakeChosenWorker.FindOrCreateContainerReturns(fakeContainer, nil)
				fakeContainer.PropertyReturns("0", fmt.Errorf("property not found"))
			})

			It("calls resource.Put with the correct ctx, processSpec and container", func() {
				actualCtx, actualProcessSpec, actualContainer := fakeResource.PutArgsForCall(0)
				Expect(actualCtx).To(Equal(ctx))
				Expect(actualProcessSpec).To(Equal(fakeProcessSpec))
				Expect(actualContainer).To(Equal(fakeContainer))
			})

			Context("when PUT returns an error", func() {

				Context("when the error is ErrResourceScriptFailed", func() {
					var (
						scriptFailErr runtime.ErrResourceScriptFailed
					)
					BeforeEach(func() {
						scriptFailErr = runtime.ErrResourceScriptFailed{
							ExitStatus: 10,
						}

						fakeResource.PutReturns(
							runtime.VersionResult{},
							scriptFailErr,
						)
					})

					It("returns a PutResult with the exit status from ErrResourceScriptFailed", func() {
						Expect(status).To(Equal(10))
						Expect(err).To(Equal(scriptFailErr))
					})
				})

				Context("when the error is NOT ErrResourceScriptFailed", func() {
					BeforeEach(func() {
						fakeResource.PutReturns(
							runtime.VersionResult{},
							disasterErr,
						)
					})

					It("returns a PutResult with status -1 and the error", func() {
						Expect(status).To(Equal(-1))
						Expect(err).To(Equal(disasterErr))
					})

				})
			})

			Context("when PUT succeeds", func() {
				var expectedVersionResult runtime.VersionResult
				BeforeEach(func() {
					expectedVersionResult = runtime.VersionResult{
						Version:  atc.Version(map[string]string{"foo": "bar"}),
						Metadata: nil,
					}

					fakeResource.PutReturns(expectedVersionResult, nil)
				})
				It("returns the correct VersionResult and Status", func() {
					Expect(err).To(BeNil())
					Expect(status).To(Equal(0))
					Expect(versionResult).To(Equal(expectedVersionResult))
				})
			})
		})

		Context("worker.FindOrCreateContainer errored", func() {
			BeforeEach(func() {
				fakeChosenWorker.FindOrCreateContainerReturns(nil, disasterErr)
			})

			It("returns the error immediately", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disasterErr))
				Expect(versionResult).To(Equal(runtime.VersionResult{}))
			})
		})
	})
})

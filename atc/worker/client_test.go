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

var _ = FDescribe("Client", func() {
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

	Describe("RunTaskStep", func() {
		var (
			status       int
			volumeMounts []worker.VolumeMount
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
				Dir:     "some-artifact-root",
				Env:     []string{"SECURE=secret-task-param"},
				Inputs:  []worker.InputSource{},
				Outputs: worker.OutputPaths{},
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

		FIt("finds or creates a container", func() {
			Expect(fakeWorker.FindOrCreateContainerCallCount()).To(Equal(1))
			_, cancel, delegate, owner, createdMetadata, containerSpec, _ := fakeWorker.FindOrCreateContainerArgsForCall(0)
			Expect(cancel).ToNot(BeNil())
			Expect(owner).To(Equal(fakeContainerOwner))
			Expect(delegate).To(Equal(fakeDelegate))
			Expect(createdMetadata).To(Equal(db.ContainerMetadata{
				WorkingDirectory: "some-artifact-root",
				Type:             db.ContainerTypeTask,
				StepName:         "some-step",
			}))
			Expect(containerSpec).To(Equal(fakeContainerSpec))
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
					fakeMountPath1 string = "some-artifact-root/some-output-configured-path/"
					fakeMountPath2 string = "some-artifact-root/some-other-output/"
					fakeMountPath3 string = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

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

				fakeMountPath1 string = "some-artifact-root/some-output-configured-path/"
				fakeMountPath2 string = "some-artifact-root/some-other-output/"
				fakeMountPath3 string = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

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
							fakeMountPath1 string = "some-artifact-root/some-output-configured-path/"
							fakeMountPath2 string = "some-artifact-root/some-other-output/"
							fakeMountPath3 string = "some-artifact-root/some-output-configured-path-with-trailing-slash/"

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
			cancel            func()
			owner             db.ContainerOwner
			containerSpec     worker.ContainerSpec
			workerSpec        worker.WorkerSpec
			// source            atc.Source
			// params            atc.Params
			metadata          db.ContainerMetadata
			imageSpec         worker.ImageFetcherSpec
			fakeChosenWorker  *workerfakes.FakeWorker
			fakeStrategy      *workerfakes.FakeContainerPlacementStrategy
			fakeDelegate      *workerfakes.FakeImageFetchingDelegate
			fakeResourceTypes atc.VersionedResourceTypes
			fakeContainer     *workerfakes.FakeContainer
			fakeProcessSpec   runtime.ProcessSpec
			fakeResource *resourcefakes.FakeResource

			versionResult runtime.VersionResult
			status        int
			err           error

			disasterErr error
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())
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

			// source = atc.Source{"some": "super-secret-source"}
			// params = atc.Params{"some-param": "some-value"}
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

				Expect(actualDelegate).To(Equal(fakeDelegate))
				Expect(actualOwner).To(Equal(owner))
				Expect(actualContainerSpec).To(Equal(containerSpec))
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

		Context("found a container that has already exited", func() {
			var status int
			BeforeEach(func() {
				status = 8
				fakeChosenWorker.FindOrCreateContainerReturns(fakeContainer, nil)
				fakeContainer.PropertyStub = func(prop string) (result string, err error) {
					if prop == "concourse:exit-status" {
						return "8", nil
					}
					return "", errors.New("unhandled property")
				}
			})

			It("does not attach to any process", func() {
				Expect(fakeContainer.AttachCallCount()).To(BeZero())
			})

			It("returns result of container process", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(8))
			})
		})

		Context("container has not already exited", func() {
			var (
				fakeProcess         *gardenfakes.FakeProcess
				fakeProcessExitCode int

				stdoutBuf *gbytes.Buffer
				stderrBuf *gbytes.Buffer
			)

			BeforeEach(func() {
				stdoutBuf = new(gbytes.Buffer)
				stderrBuf = new(gbytes.Buffer)
				fakeProcess = new(gardenfakes.FakeProcess)
				fakeContainer.PropertyReturns("", errors.New("not exited"))
			})

			Context("found container that is already running", func() {
				var expectedVersionResult runtime.VersionResult
				BeforeEach(func() {
					fakeContainer.AttachStub = func(arg1 context.Context, arg2 string, arg3 garden.ProcessIO) (garden.Process, error) {
						_, _ = arg3.Stdout.Write([]byte(`{"version": { "foo": "bar" }}`))
						return fakeProcess, nil
					}
					expectedVersionResult = runtime.VersionResult{
						Version:  atc.Version(map[string]string{"foo": "bar"}),
						Metadata: nil,
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
				})

				Context("when the process exits successfully", func() {
					BeforeEach(func() {
						fakeProcessExitCode = 0
						fakeProcess.WaitReturns(fakeProcessExitCode, nil)
					})
					It("returns a successful result", func() {
						Expect(versionResult).To(Equal(expectedVersionResult))
						Expect(status).To(BeZero())
						Expect(err).ToNot(HaveOccurred())
					})

				})

				Context("when the process exits with an error", func() {
					disaster := errors.New("process failed")
					BeforeEach(func() {
						fakeProcessExitCode = 128 + 15
						fakeProcess.WaitReturns(fakeProcessExitCode, disaster)
					})
					It("returns an unsuccessful result", func() {
						Expect(status).To(Equal(-1))
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(disaster))
						Expect(versionResult).To(Equal(runtime.VersionResult{}))
					})

					It("returns no version results", func() {
						Expect(versionResult).To(Equal(runtime.VersionResult{}))
					})
				})

				Context("when the process exits with nonzero status", func() {
					BeforeEach(func() {
						fakeProcessExitCode = 128 + 15
						fakeProcess.WaitReturns(fakeProcessExitCode, nil)
					})
					It("returns an unsuccessful result", func() {
						Expect(status).To(Equal(fakeProcessExitCode))
						Expect(err).To(HaveOccurred())
						Expect(err).To(BeAssignableToTypeOf(runtime.ErrResourceScriptFailed{}))
						Expect(versionResult).To(Equal(runtime.VersionResult{}))
					})

					It("returns no version results", func() {
						Expect(versionResult).To(Equal(runtime.VersionResult{}))
					})
				})
			})

			Context("created a new container", func() {
				BeforeEach(func() {
					fakeContainer.AttachReturns(nil, errors.New("container not running"))
					fakeContainer.RunReturns(fakeProcess, nil)

					stdoutBuf = new(gbytes.Buffer)
					stderrBuf = new(gbytes.Buffer)
					fakeProcessSpec = runtime.ProcessSpec{
						Path:         "/opt/resource/out",
						Args:         []string{"/tmp/build/put"},
						StdoutWriter: stdoutBuf,
						StderrWriter: stderrBuf,
					}
					fakeContainer.RunStub = func(arg1 context.Context, arg2 garden.ProcessSpec, arg3 garden.ProcessIO) (garden.Process, error) {
						_, _ = arg3.Stdout.Write([]byte(`{"version": { "foo": "bar" }}`))
						return fakeProcess, nil
					}
				})

				It("runs a new process in the container", func() {
					Eventually(fakeContainer.RunCallCount()).Should(Equal(1))

					_, gardenProcessSpec, actualProcessIO := fakeContainer.RunArgsForCall(0)
					Expect(gardenProcessSpec.ID).To(Equal("resource"))
					Expect(gardenProcessSpec.Path).To(Equal(fakeProcessSpec.Path))
					Expect(gardenProcessSpec.Args).To(ConsistOf(fakeProcessSpec.Args))
					Expect(actualProcessIO.Stdout).To(Not(Equal(stdoutBuf)))
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

					// It("puts the resource with the given context", func() {
					// 	Expect(fakeResource.PutCallCount()).To(Equal(1))
					// 	putCtx, _, _, _ := fakeResource.PutArgsForCall(0)
					// 	Expect(putCtx).To(Equal(ctx))
					// })

					// It("puts the resource with the correct source and params", func() {
					// 	Expect(fakeResource.PutCallCount()).To(Equal(1))
					//
					// 	_, _, putSource, putParams := fakeResource.PutArgsForCall(0)
					// 	Expect(putSource).To(Equal(atc.Source{"some": "super-secret-source"}))
					// 	Expect(putParams).To(Equal(atc.Params{"some-param": "some-value"}))
					// })

					// It("puts the resource with the io config forwarded", func() {
					// 	Expect(fakeResource.PutCallCount()).To(Equal(1))
					//
					// 	_, ioConfig, _, _ := fakeResource.PutArgsForCall(0)
					// 	Expect(ioConfig.Stdout).To(Equal(stdoutBuf))
					// 	Expect(ioConfig.Stderr).To(Equal(stderrBuf))
					// })

					// It("runs the get resource action", func() {
					// 	Expect(fakeResource.PutCallCount()).To(Equal(1))
					// })
					It("returns a successful result", func() {
						Expect(status).To(BeZero())
						Expect(err).ToNot(HaveOccurred())
					})

					It("saves the exit status property", func() {
						Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Expect(name).To(Equal("concourse:resource-result"))
						Expect(value).To(Equal(string(`{"version": { "foo": "bar" }}`)))
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

								if name == "concourse:resource-result" {
									return disaster
								}

								return nil
							}
						})

						It("returns the error", func() {
							Expect(err).To(Equal(disaster))
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
						Expect(err).To(HaveOccurred())
						Expect(err).To(BeAssignableToTypeOf(runtime.ErrResourceScriptFailed{}))
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

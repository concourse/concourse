package gardenruntime_test

import (
	"bytes"
	"context"
	"fmt"
	"testing/fstest"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker2/gardenruntime"
	grt "github.com/concourse/concourse/atc/worker2/gardenruntime/gardenruntimetest"
	"github.com/concourse/concourse/atc/worker2/workertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Garden Worker", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Test("running a process on a newly created container", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker"),
			),
		)
		worker := scenario.Worker("worker")

		container, volumeMounts, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				Dir: "/workdir",
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		By("running a process on that container", func() {
			buf := new(bytes.Buffer)
			result, err := container.Run(
				ctx,
				runtime.ProcessSpec{
					Path: "echo",
					Args: []string{"hello world"},
				},
				runtime.ProcessIO{
					Stdout: buf,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.ExitStatus).To(Equal(0))

			Expect(buf.String()).To(Equal("hello world\n"))
		})

		By("validating the container was created", func() {
			garden := gardenServer(worker)
			Expect(garden.ContainerList).To(HaveLen(1))
			Expect(garden.ContainerList[0].Handle()).To(Equal("my-handle"))
			Expect(garden.ContainerList[0].Spec.RootFSPath).To(Equal("raw:///img/rootfs"))

			dbContainer := scenario.DB.Container(worker.Name(), db.NewFixedHandleContainerOwner("my-handle"))
			_, isCreated := dbContainer.(db.CreatedContainer)
			Expect(isCreated).To(BeTrue())
		})

		By("validating the default created volumes", func() {
			Expect(volumeMountMap(volumeMounts)).To(consistOfMap(expectMap{
				"/scratch": grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
				"/workdir": grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
			}))

			Expect(bindMountVolumes(worker, container)).To(Equal(volumeMountMap(volumeMounts)))
		})
	})

	Test("finding a pre-existing container", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithContainersCreatedInDBAndGarden(
						grt.NewContainer("existing-container"),
					),
			),
		)
		worker := scenario.Worker("worker")

		_, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("existing-container"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		By("validating the container was not created", func() {
			garden := gardenServer(worker)
			Expect(garden.ContainerList).To(HaveLen(1))
			Expect(garden.ContainerList[0].Handle()).To(Equal("existing-container"))
		})
	})

	Test("attaching to a process", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithContainersCreatedInDBAndGarden(
						grt.NewContainer("existing-container"),
					),
			),
		)
		worker := scenario.Worker("worker")

		container, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				Dir: "/workdir",
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		processSpec := runtime.ProcessSpec{
			Path: "sleep-and-echo",
			Args: []string{"200ms", "hello world"},
		}
		By("running a process on that container", func() {
			runResultCh := make(chan runtime.ProcessResult, 1)
			runBuf := new(bytes.Buffer)

			go func() {
				defer GinkgoRecover()
				result, err := container.Run(ctx, processSpec, runtime.ProcessIO{Stdout: runBuf})
				Expect(err).ToNot(HaveOccurred())
				runResultCh <- result
			}()

			Eventually(gardenContainer(container).NumProcesses).Should(Equal(1))

			By("attaching to the running process", func() {
				attachBuf := new(bytes.Buffer)
				result, err := container.Attach(ctx, processSpec, runtime.ProcessIO{Stdout: attachBuf})
				Expect(err).ToNot(HaveOccurred())

				Expect(result.ExitStatus).To(Equal(0))
				Expect((<-runResultCh).ExitStatus).To(Equal(0))

				Expect(runBuf.String()).To(Equal("hello world\n"))
				Expect(attachBuf.String()).To(Equal("hello world\n"))
			})
		})

		By("attaching to that process after it's exited", func() {
			attachBuf := new(bytes.Buffer)
			result, err := container.Attach(ctx, processSpec, runtime.ProcessIO{Stdout: attachBuf})
			Expect(err).ToNot(HaveOccurred())

			Expect(result.ExitStatus).To(Equal(0))
			Expect(attachBuf.Len()).To(Equal(0))
		})
	})

	Test("container creating in DB, missing from garden", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithDBContainersInState(grt.Creating, "not-yet-in-garden"),
			),
		)
		worker := scenario.Worker("worker")

		_, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("not-yet-in-garden"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		By("validating the container was created", func() {
			garden := gardenServer(worker)
			Expect(garden.ContainerList).To(HaveLen(1))
			Expect(garden.ContainerList[0].Handle()).To(Equal("not-yet-in-garden"))

			dbContainer := scenario.DB.Container(worker.Name(), db.NewFixedHandleContainerOwner("not-yet-in-garden"))
			_, isCreated := dbContainer.(db.CreatedContainer)
			Expect(isCreated).To(BeTrue())
		})
	})

	Test("container created in DB, missing from garden", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithDBContainersInState(grt.Created, "not-in-garden"),
			),
		)
		worker := scenario.Worker("worker")

		_, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("not-in-garden"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
			},
		)
		Expect(err).To(MatchError(garden.ContainerNotFoundError{Handle: "not-in-garden"}))
	})

	Test("fetch image from volume on same worker", func() {
		imageVolume := grt.NewVolume("local-image-volume").WithContent(fstest.MapFS{
			"metadata.json": grt.ImageMetadataFile(gardenruntime.ImageMetadata{
				Env:  []string{"FOO=bar"},
				User: "somebody",
			}),
		})
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						imageVolume,
					),
			),
		)
		worker := scenario.Worker("worker")

		container, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageVolume: imageVolume.Handle(),
				},
				Env: []string{"A=b"},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		var cowVolume *grt.Volume
		By("validating the volume was cloned", func() {
			var ok bool
			cowVolume, ok = findVolumeBy(worker, grt.StrategyEq(baggageclaim.COWStrategy{Parent: imageVolume}))
			Expect(ok).To(BeTrue())
		})

		gardenContainer := gardenContainer(container)

		By("validating the container was created with the proper rootfs + metadata", func() {
			Expect(gardenContainer.Spec.RootFSPath).To(Equal(fmt.Sprintf("raw://%s/rootfs", cowVolume.Path())))
			Expect(gardenContainer.Spec.Env).To(Equal([]string{"FOO=bar", "A=b"}))
		})

		By("running a process on the container and validating it uses the user from metadata", func() {
			_, err := container.Run(ctx,
				runtime.ProcessSpec{
					Path: "noop",
				},
				runtime.ProcessIO{},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(gardenContainer.NumProcesses()).To(Equal(1))
			Expect(gardenContainer.Processes[0].Spec.User).To(Equal("somebody"))
		})

		By("running a process overriding the image metadata user", func() {
			_, err = container.Run(ctx,
				runtime.ProcessSpec{
					Path: "noop",
					User: "somebodyelse",
				},
				runtime.ProcessIO{},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(gardenContainer.NumProcesses()).To(Equal(2))
			Expect(gardenContainer.Processes[1].Spec.User).To(Equal("somebodyelse"))
		})
	})

	Test("fetch image from resource cache volume on same worker", func() {
		imageContent := fstest.MapFS{
			"metadata.json": grt.ImageMetadataFile(gardenruntime.ImageMetadata{
				Env:  []string{"FOO=bar"},
				User: "somebody",
			}),
		}
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker1").
					WithDBContainersInState(grt.Creating, "container1").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("locally-cached-volume").
							WithContent(imageContent),
					).
					WithResourceCacheOnVolume("container1", "locally-cached-volume", "some-resource"),
				grt.NewWorker("worker2").
					WithDBContainersInState(grt.Creating, "container2").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("remote-volume").
							WithContent(imageContent),
					).
					WithResourceCacheOnVolume("container2", "remote-volume", "some-resource"),
			),
		)
		worker := scenario.Worker("worker1")

		container, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("container1"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageVolume: "remote-volume",
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		localCacheVolume, ok := findVolumeBy(worker, grt.HandleEq("locally-cached-volume"))
		Expect(ok).To(BeTrue())

		var cowVolume *grt.Volume
		By("validating the volume was cloned from the locally cached copy instead of the remote one", func() {
			var ok bool
			cowVolume, ok = findVolumeBy(worker, grt.StrategyEq(baggageclaim.COWStrategy{Parent: localCacheVolume}))
			Expect(ok).To(BeTrue())
		})

		gardenContainer := gardenContainer(container)

		By("validating the container was created with the proper rootfs", func() {
			Expect(gardenContainer.Spec.RootFSPath).To(Equal(fmt.Sprintf("raw://%s/rootfs", cowVolume.Path())))
		})
	})

	Test("fetch image from volume on different worker", func() {
		imageVolume := grt.NewVolume("remote-image-volume").WithContent(fstest.MapFS{
			"metadata.json": grt.ImageMetadataFile(gardenruntime.ImageMetadata{
				Env:  []string{"FOO=bar"},
				User: "somebody",
			}),
			"rootfs/other_file": {Data: []byte("some rootfs content")},
		})
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker1"),
				grt.NewWorker("worker2").
					WithVolumesCreatedInDBAndBaggageclaim(
						imageVolume,
					),
			),
		)
		worker := scenario.Worker("worker1")

		container, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageVolume: imageVolume.Handle(),
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		var streamedVolume *grt.Volume
		By("validating the volume was streamed", func() {
			var ok bool
			streamedVolume, ok = findVolumeBy(worker, grt.ContentEq(imageVolume.Content))
			Expect(ok).To(BeTrue())
			Expect(streamedVolume).To(grt.HaveStrategy(baggageclaim.EmptyStrategy{}))
		})

		By("validating the container was created with the proper rootfs + metadata", func() {
			gardenContainer := gardenContainer(container)
			Expect(gardenContainer.Spec.RootFSPath).To(Equal(fmt.Sprintf("raw://%s/rootfs", streamedVolume.Path())))
			Expect(gardenContainer.Spec.Env).To(Equal([]string{"FOO=bar"}))
		})
	})

	Test("image from base resource type", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker"),
			),
		)
		worker := scenario.Worker("worker")

		container, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ResourceType: dbtest.BaseResourceType,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		var cowVolume *grt.Volume
		By("validating that the volume was first imported and then COW'd", func() {
			importVolume, ok := findVolumeBy(worker, grt.StrategyEq(baggageclaim.ImportStrategy{Path: "/path/to/global/image"}))
			Expect(ok).To(BeTrue())
			cowVolume, ok = findVolumeBy(worker, grt.StrategyEq(baggageclaim.COWStrategy{Parent: importVolume}))
			Expect(ok).To(BeTrue())
		})

		By("validating the container was created with the proper rootfs + metadata", func() {
			gardenContainer := gardenContainer(container)
			Expect(gardenContainer.Spec.RootFSPath).To(Equal(fmt.Sprintf("raw://%s", cowVolume.Path())))
		})
	})

	Test("input and output volumes", func() {
		localInputVolume := grt.NewVolume("local-input")
		remoteInputVolume := grt.NewVolume("remote-input").WithContent(fstest.MapFS{
			"file1": {Data: []byte("content")},
		})
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker1").
					WithVolumesCreatedInDBAndBaggageclaim(localInputVolume),
				grt.NewWorker("worker2").
					WithVolumesCreatedInDBAndBaggageclaim(remoteInputVolume),
			),
		)
		worker := scenario.Worker("worker1")

		_, volumeMounts, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir: "/workdir",
				Inputs: []runtime.Input{
					{
						VolumeHandle:    localInputVolume.Handle(),
						DestinationPath: "/local-input",
					},
					{
						VolumeHandle:    remoteInputVolume.Handle(),
						DestinationPath: "/remote-input",
					},
				},
				Outputs: runtime.OutputPaths{
					"output": "/output",
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(volumeMountMap(volumeMounts)).To(consistOfMap(expectMap{
			"/scratch":     grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
			"/workdir":     grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
			"/local-input": grt.HaveStrategy(baggageclaim.COWStrategy{Parent: localInputVolume}),
			"/remote-input": SatisfyAll(
				grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
				grt.HaveContent(remoteInputVolume.Content),
			),
			"/output": grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
		}))
	})

	Test("input volume from resource cache on same worker", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker1").
					WithDBContainersInState(grt.Creating, "container1").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("locally-cached-volume"),
					).
					WithResourceCacheOnVolume("container1", "locally-cached-volume", "some-resource"),
				grt.NewWorker("worker2").
					WithDBContainersInState(grt.Creating, "container2").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("remote-volume"),
					).
					WithResourceCacheOnVolume("container2", "remote-volume", "some-resource"),
			),
		)
		worker := scenario.Worker("worker1")

		_, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir: "/workdir",
				Inputs: []runtime.Input{
					{
						VolumeHandle:    "remote-volume",
						DestinationPath: "/input",
					},
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		localCacheVolume, ok := findVolumeBy(worker, grt.HandleEq("locally-cached-volume"))
		Expect(ok).To(BeTrue())

		By("validating the volume was cloned from the locally cached copy instead of the remote one", func() {
			_, ok := findVolumeBy(worker, grt.StrategyEq(baggageclaim.COWStrategy{Parent: localCacheVolume}))
			Expect(ok).To(BeTrue())
		})
	})

	Test("input volume matching workdir/output", func() {
		localInputVolume := grt.NewVolume("local-input")
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithVolumesCreatedInDBAndBaggageclaim(localInputVolume),
			),
		)
		worker := scenario.Worker("worker")

		_, volumeMounts, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir: "/workdir",
				Inputs: []runtime.Input{
					{
						VolumeHandle:    localInputVolume.Handle(),
						DestinationPath: "/workdir",
					},
				},
				Outputs: runtime.OutputPaths{
					"output": "/workdir",
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(volumeMountMap(volumeMounts)).To(consistOfMap(expectMap{
			"/scratch": grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
			"/workdir": grt.HaveStrategy(baggageclaim.COWStrategy{Parent: localInputVolume}),
		}))
	})

	Test("cached paths", func() {
		scenario := Setup(
			workertest.WithBasicJob(),
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithCachedPaths("/cache-hit", "/workdir"),
			),
		)
		worker := scenario.Worker("worker")

		origCacheHitVol, ok := cacheVolume(scenario, worker, "/cache-hit")
		Expect(ok).To(BeTrue())
		origWorkdirCacheVol, ok := cacheVolume(scenario, worker, "/workdir")
		Expect(ok).To(BeTrue())

		_, volumeMounts, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir: "/workdir",
				Caches: []string{
					"/cache-hit",
					"/cache-miss",
					"/workdir",
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(volumeMountMap(volumeMounts)).To(consistOfMap(expectMap{
			"/scratch":    grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
			"/workdir":    grt.HaveStrategy(baggageclaim.COWStrategy{Parent: origWorkdirCacheVol}),
			"/cache-hit":  grt.HaveStrategy(baggageclaim.COWStrategy{Parent: origCacheHitVol}),
			"/cache-miss": grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
		}))
	})

	Test("certs bind mount", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker"),
			),
		)
		worker := scenario.Worker("worker")

		container, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir:            "/workdir",
				CertsBindMount: true,
			},
		)
		Expect(err).ToNot(HaveOccurred())

		Expect(bindMountVolumes(worker, container)).To(consistOfMap(expectMap{
			"/scratch": grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
			"/workdir": grt.HaveStrategy(baggageclaim.EmptyStrategy{}),
			"/etc/ssl/certs": grt.HaveStrategy(baggageclaim.ImportStrategy{
				Path:           dbtest.CertsPath,
				FollowSymlinks: true,
			}),
		}))

		By("validating that the certs volume is mounted as read only", func() {
			Expect(bindMount(container, "/etc/ssl/certs").Mode).To(Equal(garden.BindMountModeRO))
		})

		By("validating that other volumes are mounted as read write", func() {
			Expect(bindMount(container, "/scratch").Mode).To(Equal(garden.BindMountModeRW))
		})
	})

	Test("privileged image produces privileged volumes", func() {
		imageVolume := grt.NewVolume("image-volume").WithContent(fstest.MapFS{
			"metadata.json": grt.ImageMetadataFile(gardenruntime.ImageMetadata{}),
		})
		inputVolume := grt.NewVolume("input")
		scenario := Setup(
			workertest.WithBasicJob(),
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						imageVolume,
						inputVolume,
					),
			),
		)
		worker := scenario.Worker("worker")

		container, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				Dir: "/workdir",
				ImageSpec: runtime.ImageSpec{
					ImageVolume: imageVolume.Handle(),
					Privileged:  true,
				},
				Inputs: []runtime.Input{
					{
						VolumeHandle:    inputVolume.Handle(),
						DestinationPath: "/input",
					},
				},
				Outputs: runtime.OutputPaths{
					"output": "/output",
				},
				Caches: []string{"/cache"},

				CertsBindMount: true,
			},
		)
		Expect(err).ToNot(HaveOccurred())

		By("validating the image volume is privileged", func() {
			cowImageVolume, ok := findVolumeBy(worker, grt.StrategyEq(baggageclaim.COWStrategy{Parent: imageVolume}))
			Expect(ok).To(BeTrue())
			Expect(cowImageVolume).To(grt.BePrivileged())
		})

		By("validating the container mounts are privileged", func() {
			Expect(bindMountVolumes(worker, container)).To(consistOfMap(expectMap{
				"/scratch": grt.BePrivileged(),
				"/workdir": grt.BePrivileged(),
				"/input":   grt.BePrivileged(),
				"/output":  grt.BePrivileged(),
				"/cache":   grt.BePrivileged(),
				// The certs volume shouldn't be privileged
				"/etc/ssl/certs": Not(grt.BePrivileged()),
			}))
		})
	})

	Test("unprivileged image produces unprivileged volumes", func() {
		imageVolume := grt.NewVolume("image-volume").WithContent(fstest.MapFS{
			"metadata.json": grt.ImageMetadataFile(gardenruntime.ImageMetadata{}),
		})
		inputVolume := grt.NewVolume("input")
		scenario := Setup(
			workertest.WithBasicJob(),
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						imageVolume,
						inputVolume,
					),
			),
		)
		worker := scenario.Worker("worker")

		container, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				TeamID:   scenario.TeamID,
				JobID:    scenario.JobID,
				StepName: scenario.StepName,

				Dir: "/workdir",
				ImageSpec: runtime.ImageSpec{
					ImageVolume: imageVolume.Handle(),
					Privileged:  false,
				},
				Inputs: []runtime.Input{
					{
						VolumeHandle:    inputVolume.Handle(),
						DestinationPath: "/input",
					},
				},
				Outputs: runtime.OutputPaths{
					"output": "/output",
				},
				Caches: []string{"/cache"},

				CertsBindMount: true,
			},
		)
		Expect(err).ToNot(HaveOccurred())

		By("validating the image volume is unprivileged", func() {
			cowImageVolume, ok := findVolumeBy(worker, grt.StrategyEq(baggageclaim.COWStrategy{Parent: imageVolume}))
			Expect(ok).To(BeTrue())
			Expect(cowImageVolume).ToNot(grt.BePrivileged())
		})

		By("validating the container mounts are unprivileged", func() {
			Expect(bindMountVolumes(worker, container)).To(consistOfMap(expectMap{
				"/scratch":       Not(grt.BePrivileged()),
				"/workdir":       Not(grt.BePrivileged()),
				"/input":         Not(grt.BePrivileged()),
				"/output":        Not(grt.BePrivileged()),
				"/cache":         Not(grt.BePrivileged()),
				"/etc/ssl/certs": Not(grt.BePrivileged()),
			}))
		})
	})

	Test("missing input volume", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker"),
			),
		)
		worker := scenario.Worker("worker")

		_, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-handle"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir: "/workdir",
				Inputs: []runtime.Input{
					{
						VolumeHandle:    "missing-volume",
						DestinationPath: "/volume",
					},
				},
			},
		)
		Expect(err).To(MatchError(MatchRegexp(`input .* not found`)))
	})

	Test("container volume creating, but not in baggageclaim", func() {
		scenario := Setup(
			workertest.WithBasicJob(),
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithDBContainersInState(grt.Creating, "my-container").
					WithDBContainerVolumesInState(grt.Creating, "my-container", "/scratch"),
			),
		)
		worker := scenario.Worker("worker")

		_, volumeMounts, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-container"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				TeamID: scenario.TeamID,
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir: "/workdir",
			},
		)
		Expect(err).ToNot(HaveOccurred())

		By("validating the container was created", func() {
			_, ok := volumeMountMap(volumeMounts)["/scratch"]
			Expect(ok).To(BeTrue())
		})
	})

	Test("container volume created, but not in baggageclaim", func() {
		scenario := Setup(
			workertest.WithBasicJob(),
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithDBContainersInState(grt.Creating, "my-container").
					WithDBContainerVolumesInState(grt.Created, "my-container", "/scratch"),
			),
		)
		worker := scenario.Worker("worker")

		_, _, err := worker.FindOrCreateContainer(
			ctx,
			db.NewFixedHandleContainerOwner("my-container"),
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				TeamID: scenario.TeamID,
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir: "/workdir",
			},
		)
		Expect(err).To(MatchError(MatchRegexp(`volume .* disappeared from worker`)))
	})

	Test("retries when cannot acquire volume create lock", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker").
					WithDBContainersInState(grt.Creating, "my-container").
					WithDBContainerVolumesInState(grt.Creating, "my-container", "/scratch"),
			),
		)
		worker := scenario.Worker("worker")

		var scratchDBVolume db.CreatingVolume
		var volumeLock lock.Lock
		By("acquiring the volume creating lock", func() {
			scratchDBVolume, _ = scenario.ContainerVolume(worker.Name(), "my-container", "/scratch")

			logger := lagertest.NewTestLogger("dummy")

			var acquired bool
			var err error
			volumeLock, acquired, err = lockFactory.Acquire(logger, lock.NewVolumeCreatingLockID(scratchDBVolume.ID()))
			Expect(err).ToNot(HaveOccurred())
			Expect(acquired).To(BeTrue())
		})

		done := make(chan error, 1)
		go func() {
			_, _, err := worker.FindOrCreateContainer(
				ctx,
				db.NewFixedHandleContainerOwner("my-container"),
				db.ContainerMetadata{},
				runtime.ContainerSpec{
					TeamID: scenario.TeamID,
					ImageSpec: runtime.ImageSpec{
						ImageURL: "raw:///img/rootfs",
					},
					Dir: "/workdir",
				},
			)
			done <- err
		}()

		volumes := func() []*grt.Volume {
			return baggageclaimServer(worker).Volumes
		}
		By("validating the volume is not created in baggageclaim", func() {
			Consistently(volumes).ShouldNot(ContainElement(grt.HaveHandle(scratchDBVolume.Handle())))
		})

		By("unlocking the volume creating lock", func() {
			err := volumeLock.Release()
			Expect(err).ToNot(HaveOccurred())
		})

		By("validating volume creation proceeds", func() {
			Eventually(done, 2*time.Second).Should(Receive(BeNil()))
			Expect(volumes()).Should(ContainElement(grt.HaveHandle(scratchDBVolume.Handle())))
		})
	})

	Test("creating container in garden fails", func() {
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("worker"),
			),
		)
		worker := scenario.Worker("worker")

		containerOwner := db.NewFixedHandleContainerOwner("fail-to-create")
		_, _, err := worker.FindOrCreateContainer(
			ctx,
			containerOwner,
			db.ContainerMetadata{},
			runtime.ContainerSpec{
				ImageSpec: runtime.ImageSpec{
					ImageURL: "raw:///img/rootfs",
				},
				Dir: "/workdir",
			},
		)
		Expect(err).To(HaveOccurred())

		By("validating container is marked as failed", func() {
			// failed containers aren't returned by db.Worker.FindContainer
			_, isDBContainerFound := scenario.DB.FindContainer(worker.Name(), containerOwner)
			Expect(isDBContainerFound).To(BeFalse())
		})
	})
})

type expectMap map[string]interface{}

func consistOfMap(expect expectMap) types.GomegaMatcher {
	matchers := []types.GomegaMatcher{HaveLen(len(expect))}
	for k, v := range expect {
		matchers = append(matchers, HaveKeyWithValue(k, v))
	}
	return SatisfyAll(matchers...)
}

func volumeMountMap(volumeMounts []runtime.VolumeMount) map[string]*grt.Volume {
	mounts := make(map[string]*grt.Volume)
	for _, mnt := range volumeMounts {
		mounts[mnt.MountPath] = mnt.Volume.(gardenruntime.Volume).BaggageclaimVolume().(*grt.Volume)
	}

	return mounts
}

func bindMount(container runtime.Container, path string) garden.BindMount {
	for _, mnt := range gardenContainer(container).Spec.BindMounts {
		if mnt.DstPath == path {
			return mnt
		}
	}

	Fail("missing mount " + path)
	panic("unreachable")
}

func bindMountVolumes(worker runtime.Worker, container runtime.Container) map[string]*grt.Volume {
	bindMounts := make(map[string]*grt.Volume)
	for _, mnt := range gardenContainer(container).Spec.BindMounts {
		bcVolume, ok := findVolumeBy(worker, grt.PathEq(mnt.SrcPath))
		Expect(ok).To(BeTrue(), "bind mount volume not found in baggageclaim")
		bindMounts[mnt.DstPath] = bcVolume
	}
	return bindMounts
}

func gardenContainer(container runtime.Container) *grt.Container {
	gardenRuntimeContainer, ok := container.(gardenruntime.Container)
	Expect(ok).To(BeTrue(), "must be called on a gardenruntime.Container")
	return gardenRuntimeContainer.GardenContainer.(*grt.Container)
}

func gardenServer(worker runtime.Worker) *grt.Garden {
	gardenWorker, ok := worker.(*gardenruntime.Worker)
	Expect(ok).To(BeTrue(), "must be called on a *gardenruntime.Worker")

	garden := gardenWorker.GardenClient().(*grt.Garden)
	return garden
}

func baggageclaimServer(worker runtime.Worker) *grt.Baggageclaim {
	gardenWorker, ok := worker.(*gardenruntime.Worker)
	Expect(ok).To(BeTrue(), "must be called on a *gardenruntime.Worker")

	baggageclaim := gardenWorker.BaggageclaimClient().(*grt.Baggageclaim)
	return baggageclaim
}

func findVolumeBy(worker runtime.Worker, pred func(*grt.Volume) bool) (*grt.Volume, bool) {
	baggageclaim := baggageclaimServer(worker)

	volumes := baggageclaim.FilteredVolumes(pred)
	if len(volumes) == 0 {
		return nil, false
	}
	Expect(volumes).To(HaveLen(1), "volume not uniquely specified")
	return volumes[0], true
}

func cacheVolume(scenario *workertest.Scenario, worker runtime.Worker, path string) (*grt.Volume, bool) {
	cacheDBVolume := scenario.WorkerTaskCacheVolume(worker.Name(), path)
	return findVolumeBy(worker, grt.HandleEq(cacheDBVolume.Handle()))
}

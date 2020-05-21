package worker_test

import (
	"bytes"
	"code.cloudfoundry.org/garden"
	"context"
	"github.com/concourse/concourse/atc/metric"
	"time"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/compression/compressionfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunTaskStep", func() {
	var (
		subject    worker.Client
		taskResult worker.TaskResult
		err        error

		outputBuffer *bytes.Buffer
		ctx          context.Context

		fakeWorker           *workerfakes.FakeWorker
		fakePool             *workerfakes.FakePool
		fakeTaskProcessSpec  runtime.ProcessSpec
		fakeLock             *lockfakes.FakeLock
		fakeProvider         *workerfakes.FakeWorkerProvider
		fakeCompression      *compressionfakes.FakeCompression
		fakeContainerOwner   db.ContainerOwner
		fakeContainerSpec    worker.ContainerSpec
		fakeWorkerSpec       worker.WorkerSpec
		fakeStrategy         *workerfakes.FakeContainerPlacementStrategy
		fakeMetadata         db.ContainerMetadata
		fakeImageFetcherSpec worker.ImageFetcherSpec
		fakeEventDelegate    *runtimefakes.FakeStartingEventDelegate
		fakeLockFactory      *lockfakes.FakeLockFactory
	)

	Context("assign task when", func() {
		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			outputBuffer = new(bytes.Buffer)
			ctx, _ = context.WithCancel(context.Background())

			fakePool = new(workerfakes.FakePool)
			fakeProvider = new(workerfakes.FakeWorkerProvider)
			fakeCompression = new(compressionfakes.FakeCompression)
			fakeContainerOwner = containerOwnerDummy()
			fakeContainerSpec = workerContainerDummy()
			fakeWorkerSpec = worker.WorkerSpec{}
			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
			fakeMetadata = containerMetadataDummy()
			fakeImageFetcherSpec = imageFetcherDummy()
			fakeTaskProcessSpec = processSpecDummy(outputBuffer)
			fakeEventDelegate = new(runtimefakes.FakeStartingEventDelegate)
			fakeLockFactory = new(lockfakes.FakeLockFactory)
			fakeWorker = fakeWorkerStub()
			fakeLock = new(lockfakes.FakeLock)

			fakeStrategy.ModifiesActiveTasksReturns(true)
			fakeLockFactory.AcquireReturns(fakeLock, true, nil)
		})

		JustBeforeEach(func() {
			workerInterval := 250 * time.Millisecond
			workerStatusInterval := 500 * time.Millisecond

			subject = worker.NewClient(
				fakePool,
				fakeProvider,
				fakeCompression,
				workerInterval,
				workerStatusInterval)
		})

		Context("worker is available", func() {
			BeforeEach(func() {
				fakePool.ContainerInWorkerReturns(false, nil)
				fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)
			})

			JustBeforeEach(func() {
				taskResult, err = subject.RunTaskStep(ctx,
					logger,
					fakeContainerOwner,
					fakeContainerSpec,
					fakeWorkerSpec,
					fakeStrategy,
					fakeMetadata,
					fakeImageFetcherSpec,
					fakeTaskProcessSpec,
					fakeEventDelegate,
					fakeLockFactory)
			})

			It("returns result of container process", func() {
				Expect(err).To(BeNil())
				Expect(taskResult).To(Not(BeNil()))
				Expect(taskResult.ExitStatus).To(BeZero())
			})

			It("releases lock acquired", func() {
				Expect(fakeLock.ReleaseCallCount()).To(Equal(fakeLockFactory.AcquireCallCount()))
			})

			It("increases the active task count", func() {
				Expect(fakeWorker.IncreaseActiveTasksCallCount()).To(Equal(1))
				Expect(fakeLock.ReleaseCallCount()).To(Equal(fakeLockFactory.AcquireCallCount()))
			})

			Context("when the container is already present on the worker", func() {
				BeforeEach(func() {
					fakePool.ContainerInWorkerReturns(true, nil)
				})

				It("does not increase the active task count", func() {
					Expect(fakeWorker.IncreaseActiveTasksCallCount()).To(Equal(0))
					Expect(fakeLock.ReleaseCallCount()).To(Equal(fakeLockFactory.AcquireCallCount()))
				})

			})
		})

		Context("waiting for worker to be available", func() {
			BeforeEach(func() {
				fakePool.FindOrChooseWorkerForContainerReturnsOnCall(0, nil, nil)
				fakePool.FindOrChooseWorkerForContainerReturnsOnCall(1, nil, nil)
				fakePool.FindOrChooseWorkerForContainerReturnsOnCall(2, nil, nil)
				fakePool.FindOrChooseWorkerForContainerReturnsOnCall(3, fakeWorker, nil)
			})

			JustBeforeEach(func() {
				taskResult, err = subject.RunTaskStep(ctx,
					logger,
					fakeContainerOwner,
					fakeContainerSpec,
					fakeWorkerSpec,
					fakeStrategy,
					fakeMetadata,
					fakeImageFetcherSpec,
					fakeTaskProcessSpec,
					fakeEventDelegate,
					fakeLockFactory)
			})

			It("returns result of container process", func() {
				Expect(err).To(BeNil())
				Expect(taskResult).To(Not(BeNil()))
				Expect(taskResult.ExitStatus).To(BeZero())
			})

			It("releases lock properly", func() {
				Expect(fakeLock.ReleaseCallCount()).To(Equal(fakeLockFactory.AcquireCallCount()))
			})

			It("task waiting metrics is gauged", func() {
				Eventually(metric.TasksWaiting.Max(), 2*time.Second).Should(Equal(float64(1)))
				Eventually(metric.TasksWaiting.Max(), 2*time.Second).Should(Equal(float64(0)))
			})

			It("writes status to output writer", func() {
				output := outputBuffer.String()
				Expect(output).To(ContainSubstring("All workers are busy at the moment, please stand-by.\n"))
				Expect(output).To(ContainSubstring("Found a free worker after waiting"))
			})
		})
	})
})

func processSpecDummy(outputBuffer *bytes.Buffer) runtime.ProcessSpec {
	return runtime.ProcessSpec{
		Path:         "/some/path",
		Args:         []string{"some", "args"},
		Dir:          "/some/dir",
		StdoutWriter: outputBuffer,
		StderrWriter: new(bytes.Buffer),
	}
}

func imageFetcherDummy() worker.ImageFetcherSpec {
	return worker.ImageFetcherSpec{
		Delegate:      new(execfakes.FakeTaskDelegate),
		ResourceTypes: atc.VersionedResourceTypes{},
	}
}

func containerMetadataDummy() db.ContainerMetadata {
	return db.ContainerMetadata{
		WorkingDirectory: "some-artifact-root",
		Type:             db.ContainerTypeTask,
		StepName:         "some-step",
	}
}

func workerContainerDummy() worker.ContainerSpec {
	cpu := uint64(1024)
	memory := uint64(1024)

	return worker.ContainerSpec{
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
		Dir:            "some-artifact-root",
		Env:            []string{"SECURE=secret-task-param"},
		ArtifactByPath: map[string]runtime.Artifact{},
		Inputs:         []worker.InputSource{},
		Outputs:        worker.OutputPaths{},
	}
}

func containerOwnerDummy() db.ContainerOwner {
	return db.NewBuildStepContainerOwner(
		1234,
		atc.PlanID(42),
		123,
	)
}

func fakeWorkerStub() *workerfakes.FakeWorker {
	fakeContainer := new(workerfakes.FakeContainer)
	fakeContainer.PropertiesReturns(garden.Properties{"concourse:exit-status": "0"}, nil)

	fakeWorker := new(workerfakes.FakeWorker)
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
	return fakeWorker
}

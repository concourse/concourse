package engine_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	execfakes "github.com/concourse/atc/exec/fakes"
)

var _ = Describe("Exec Engine Locations", func() {

	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *fakes.FakeBuildDelegateFactory
		fakeDB              *fakes.FakeEngineDB

		execEngine engine.Engine

		buildModel       db.Build
		expectedMetadata engine.StepMetadata
		logger           *lagertest.TestLogger

		fakeDelegate *fakes.FakeBuildDelegate
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(fakes.FakeBuildDelegateFactory)
		fakeDB = new(fakes.FakeEngineDB)

		execEngine = engine.NewExecEngine(fakeFactory, fakeDelegateFactory, fakeDB)

		fakeDelegate = new(fakes.FakeBuildDelegate)
		fakeDelegateFactory.DelegateReturns(fakeDelegate)

		buildModel = db.Build{
			ID:           84,
			Name:         "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
		}

		expectedMetadata = engine.StepMetadata{
			BuildID:      84,
			BuildName:    "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
		}
	})

	Describe("Resume", func() {
		Context("get with nil location", func() {
			var (
				getStepFactory *execfakes.FakeStepFactory
				getStep        *execfakes.FakeStep

				fakeDelegate    *fakes.FakeBuildDelegate
				fakeGetDelegate *execfakes.FakeGetDelegate

				plan atc.Plan
			)

			BeforeEach(func() {
				getStepFactory = new(execfakes.FakeStepFactory)
				getStep = new(execfakes.FakeStep)
				getStep.ResultStub = successResult(true)
				getStepFactory.UsingReturns(getStep)
				fakeFactory.GetReturns(getStepFactory)

				fakeDelegate = new(fakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakeGetDelegate = new(execfakes.FakeGetDelegate)
				fakeDelegate.InputDelegateReturns(fakeGetDelegate)

				plan = atc.Plan{
					Location: nil,
					Get: &atc.GetPlan{
						Name: "some input",
					},
				}
			})

			It("constructs the step correctly", func() {
				build, err := execEngine.CreateBuild(buildModel, plan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)

				Expect(fakeFactory.GetCallCount()).To(Equal(1))
				logger, metadata, sourceName, workerID, delegate, _, _, _, _ := fakeFactory.GetArgsForCall(0)
				Expect(logger).ToNot(BeNil())
				Expect(metadata).To(Equal(expectedMetadata))
				Expect(sourceName).To(Equal(exec.SourceName("some input")))
				Expect(workerID).To(Equal(worker.Identifier{
					ContainerIdentifier: db.ContainerIdentifier{
						BuildID: 84,
						Type:    db.ContainerTypeGet,
						Name:    "some input",
					},
				}))

				Expect(delegate).To(Equal(fakeGetDelegate))
				_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
				Expect(location).NotTo(BeNil())
			})
		})

		Context("put with nil location", func() {
			var (
				putStepFactory *execfakes.FakeStepFactory
				putStep        *execfakes.FakeStep

				fakeDelegate    *fakes.FakeBuildDelegate
				fakePutDelegate *execfakes.FakePutDelegate

				plan atc.Plan
			)

			BeforeEach(func() {
				putStepFactory = new(execfakes.FakeStepFactory)
				putStep = new(execfakes.FakeStep)
				putStep.ResultStub = successResult(true)
				putStepFactory.UsingReturns(putStep)
				fakeFactory.PutReturns(putStepFactory)

				fakeDelegate = new(fakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakePutDelegate = new(execfakes.FakePutDelegate)
				fakeDelegate.OutputDelegateReturns(fakePutDelegate)

				plan = atc.Plan{
					Location: nil,
					Put: &atc.PutPlan{
						Name: "some output",
					},
				}
			})

			It("constructs the step correctly", func() {
				build, err := execEngine.CreateBuild(buildModel, plan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)

				Expect(fakeFactory.PutCallCount()).To(Equal(1))
				logger, metadata, workerID, delegate, _, _, _ := fakeFactory.PutArgsForCall(0)
				Expect(logger).ToNot(BeNil())
				Expect(metadata).To(Equal(expectedMetadata))
				Expect(workerID).To(Equal(worker.Identifier{
					ContainerIdentifier: db.ContainerIdentifier{
						BuildID: 84,
						Type:    db.ContainerTypePut,
						Name:    "some output",
					},
				}))

				Expect(delegate).To(Equal(fakePutDelegate))
				_, _, location := fakeDelegate.OutputDelegateArgsForCall(0)
				Expect(location).NotTo(BeNil())
			})
		})

		Context("task with nil location", func() {
			var (
				taskStepFactory *execfakes.FakeStepFactory
				taskStep        *execfakes.FakeStep

				fakeDelegate          *fakes.FakeBuildDelegate
				fakeExecutionDelegate *execfakes.FakeTaskDelegate

				plan atc.Plan
			)

			BeforeEach(func() {
				taskStepFactory = new(execfakes.FakeStepFactory)
				taskStep = new(execfakes.FakeStep)
				taskStep.ResultStub = successResult(true)
				taskStepFactory.UsingReturns(taskStep)
				fakeFactory.TaskReturns(taskStepFactory)

				fakeDelegate = new(fakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakeExecutionDelegate = new(execfakes.FakeTaskDelegate)
				fakeDelegate.ExecutionDelegateReturns(fakeExecutionDelegate)

				plan = atc.Plan{
					Location: nil,
					Task: &atc.TaskPlan{
						Name:       "some task",
						ConfigPath: "some-path-to-config",
					},
				}
			})

			It("constructs the step correctly", func() {
				build, err := execEngine.CreateBuild(buildModel, plan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)

				Expect(fakeFactory.TaskCallCount()).To(Equal(1))
				logger, sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(0)
				Expect(logger).ToNot(BeNil())
				Expect(sourceName).To(Equal(exec.SourceName("some task")))
				Expect(workerID).To(Equal(worker.Identifier{
					ContainerIdentifier: db.ContainerIdentifier{
						BuildID: 84,
						Type:    db.ContainerTypeTask,
						Name:    "some task",
					},
				}))

				Expect(delegate).To(Equal(fakeExecutionDelegate))
				_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(0)
				Expect(location).NotTo(BeNil())
			})
		})

		Context("dependent get with nil location", func() {
			var (
				getStepFactory *execfakes.FakeStepFactory
				getStep        *execfakes.FakeStep

				fakeDelegate    *fakes.FakeBuildDelegate
				fakeGetDelegate *execfakes.FakeGetDelegate

				plan atc.Plan
			)

			BeforeEach(func() {
				getStepFactory = new(execfakes.FakeStepFactory)
				getStep = new(execfakes.FakeStep)
				getStep.ResultStub = successResult(true)
				getStepFactory.UsingReturns(getStep)
				fakeFactory.DependentGetReturns(getStepFactory)

				fakeDelegate = new(fakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakeGetDelegate = new(execfakes.FakeGetDelegate)
				fakeDelegate.InputDelegateReturns(fakeGetDelegate)

				plan = atc.Plan{
					Location: nil,
					DependentGet: &atc.DependentGetPlan{
						Name: "some input",
					},
				}
			})

			It("constructs the step correctly", func() {
				build, err := execEngine.CreateBuild(buildModel, plan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)

				Expect(fakeFactory.DependentGetCallCount()).To(Equal(1))
				logger, metadata, sourceName, workerID, delegate, _, _, _ := fakeFactory.DependentGetArgsForCall(0)
				Expect(logger).ToNot(BeNil())
				Expect(metadata).To(Equal(expectedMetadata))
				Expect(sourceName).To(Equal(exec.SourceName("some input")))
				Expect(workerID).To(Equal(worker.Identifier{
					ContainerIdentifier: db.ContainerIdentifier{
						BuildID: 84,
						Type:    db.ContainerTypeGet,
						Name:    "some input",
					},
				}))

				Expect(delegate).To(Equal(fakeGetDelegate))
				_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
				Expect(location).NotTo(BeNil())
			})
		})
	})

})

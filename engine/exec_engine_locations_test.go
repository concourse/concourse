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
				Ω(err).ShouldNot(HaveOccurred())
				build.Resume(logger)

				Ω(fakeFactory.GetCallCount()).Should(Equal(1))
				logger, metadata, sourceName, workerID, delegate, _, _, _, _ := fakeFactory.GetArgsForCall(0)
				Ω(logger).ShouldNot(BeNil())
				Ω(metadata).Should(Equal(expectedMetadata))
				Ω(sourceName).Should(Equal(exec.SourceName("some input")))
				Ω(workerID).Should(Equal(worker.Identifier{
					BuildID: 84,
					Type:    db.ContainerTypeGet,
					Name:    "some input",
				}))

				Ω(delegate).Should(Equal(fakeGetDelegate))
				_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
				Ω(location).ShouldNot(BeNil())
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
				Ω(err).ShouldNot(HaveOccurred())
				build.Resume(logger)

				Ω(fakeFactory.PutCallCount()).Should(Equal(1))
				logger, metadata, workerID, delegate, _, _, _ := fakeFactory.PutArgsForCall(0)
				Ω(logger).ShouldNot(BeNil())
				Ω(metadata).Should(Equal(expectedMetadata))
				Ω(workerID).Should(Equal(worker.Identifier{
					BuildID: 84,
					Type:    db.ContainerTypePut,
					Name:    "some output",
				}))

				Ω(delegate).Should(Equal(fakePutDelegate))
				_, _, location := fakeDelegate.OutputDelegateArgsForCall(0)
				Ω(location).ShouldNot(BeNil())
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
				Ω(err).ShouldNot(HaveOccurred())
				build.Resume(logger)

				Ω(fakeFactory.TaskCallCount()).Should(Equal(1))
				logger, sourceName, workerID, delegate, _, _, _ := fakeFactory.TaskArgsForCall(0)
				Ω(logger).ShouldNot(BeNil())
				Ω(sourceName).Should(Equal(exec.SourceName("some task")))
				Ω(workerID).Should(Equal(worker.Identifier{
					BuildID: 84,
					Type:    db.ContainerTypeTask,
					Name:    "some task",
				}))

				Ω(delegate).Should(Equal(fakeExecutionDelegate))
				_, _, location := fakeDelegate.ExecutionDelegateArgsForCall(0)
				Ω(location).ShouldNot(BeNil())
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
				Ω(err).ShouldNot(HaveOccurred())
				build.Resume(logger)

				Ω(fakeFactory.DependentGetCallCount()).Should(Equal(1))
				logger, metadata, sourceName, workerID, delegate, _, _, _ := fakeFactory.DependentGetArgsForCall(0)
				Ω(logger).ShouldNot(BeNil())
				Ω(metadata).Should(Equal(expectedMetadata))
				Ω(sourceName).Should(Equal(exec.SourceName("some input")))
				Ω(workerID).Should(Equal(worker.Identifier{
					BuildID: 84,
					Type:    db.ContainerTypeGet,
					Name:    "some input",
				}))

				Ω(delegate).Should(Equal(fakeGetDelegate))
				_, _, location := fakeDelegate.InputDelegateArgsForCall(0)
				Ω(location).ShouldNot(BeNil())
			})
		})
	})

})

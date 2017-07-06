package engine_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/enginefakes"

	"github.com/concourse/atc/exec/execfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Exec Engine with Try", func() {
	var (
		fakeFactory         *execfakes.FakeFactory
		fakeDelegateFactory *enginefakes.FakeBuildDelegateFactory

		execEngine engine.Engine

		build              *dbfakes.FakeBuild
		expectedTeamID     = 1111
		expectedPipelineID = 2222
		expectedJobID      = 3333
		expectedBuildID    = 4444
		expectedMetadata   engine.StepMetadata
		logger             *lagertest.TestLogger

		fakeDelegate *enginefakes.FakeBuildDelegate
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeFactory = new(execfakes.FakeFactory)
		fakeDelegateFactory = new(enginefakes.FakeBuildDelegateFactory)

		execEngine = engine.NewExecEngine(
			fakeFactory,
			fakeDelegateFactory,
			"http://example.com",
		)

		fakeDelegate = new(enginefakes.FakeBuildDelegate)
		fakeDelegateFactory.DelegateReturns(fakeDelegate)

		build = new(dbfakes.FakeBuild)
		build.IDReturns(expectedBuildID)
		build.NameReturns("42")
		build.JobNameReturns("some-job")
		build.JobIDReturns(expectedJobID)
		build.PipelineNameReturns("some-pipeline")
		build.PipelineIDReturns(expectedPipelineID)
		build.TeamNameReturns("some-team")
		build.TeamIDReturns(expectedTeamID)

		expectedMetadata = engine.StepMetadata{
			BuildID:      expectedBuildID,
			BuildName:    "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
			TeamName:     "some-team",
			ExternalURL:  "http://example.com",
		}
	})

	Context("running try steps", func() {
		var (
			taskStepFactory *execfakes.FakeStepFactory
			taskStep        *execfakes.FakeStep

			inputStepFactory *execfakes.FakeStepFactory
			inputStep        *execfakes.FakeStep
		)

		BeforeEach(func() {
			taskStepFactory = new(execfakes.FakeStepFactory)
			taskStep = new(execfakes.FakeStep)
			taskStep.SucceededReturns(true)
			taskStepFactory.UsingReturns(taskStep)
			fakeFactory.TaskReturns(taskStepFactory)

			inputStepFactory = new(execfakes.FakeStepFactory)
			inputStep = new(execfakes.FakeStep)
			inputStep.SucceededReturns(true)
			inputStepFactory.UsingReturns(inputStep)
			fakeFactory.GetReturns(inputStepFactory)
		})

		Context("constructing steps", func() {
			var (
				fakeDelegate     *enginefakes.FakeBuildDelegate
				fakeGetDelegate  *execfakes.FakeActionsBuildEventsDelegate
				fakeTaskDelegate *execfakes.FakeActionsBuildEventsDelegate
				inputPlan        atc.Plan
				planFactory      atc.PlanFactory
			)

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				fakeDelegate = new(enginefakes.FakeBuildDelegate)
				fakeDelegateFactory.DelegateReturns(fakeDelegate)

				fakeGetDelegate = new(execfakes.FakeActionsBuildEventsDelegate)
				fakeTaskDelegate = new(execfakes.FakeActionsBuildEventsDelegate)

				fakeDelegate.DBActionsBuildEventsDelegateReturnsOnCall(0, fakeGetDelegate)
				fakeDelegate.DBActionsBuildEventsDelegateReturnsOnCall(1, fakeTaskDelegate)

				inputPlan = planFactory.NewPlan(atc.GetPlan{
					Name: "some-input",
				})

				plan := planFactory.NewPlan(atc.TryPlan{
					Step: inputPlan,
				})

				build, err := execEngine.CreateBuild(logger, build, plan)
				Expect(err).NotTo(HaveOccurred())
				build.Resume(logger)
			})

			It("constructs the step correctly", func() {
				Expect(fakeFactory.GetCallCount()).To(Equal(1))
				logger, plan, dbBuild, stepMetadata, containerMetadata, _, _ := fakeFactory.GetArgsForCall(0)
				Expect(logger).NotTo(BeNil())
				Expect(dbBuild).To(Equal(build))
				Expect(plan).To(Equal(inputPlan))
				Expect(stepMetadata).To(Equal(expectedMetadata))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeGet,
					StepName:     "some-input",
					PipelineID:   expectedPipelineID,
					PipelineName: "some-pipeline",
					JobID:        expectedJobID,
					JobName:      "some-job",
					BuildID:      expectedBuildID,
					BuildName:    "42",
				}))
				originID := fakeDelegate.DBActionsBuildEventsDelegateArgsForCall(0)
				Expect(originID).To(Equal(inputPlan.ID))
			})
		})

		Context("when the inner step fails", func() {
			var planFactory atc.PlanFactory

			BeforeEach(func() {
				planFactory = atc.NewPlanFactory(123)
				inputStep.SucceededReturns(false)
			})

			It("runs the next step", func() {
				plan := planFactory.NewPlan(atc.OnSuccessPlan{
					Step: planFactory.NewPlan(atc.TryPlan{
						Step: planFactory.NewPlan(atc.GetPlan{
							Name: "some-input",
						}),
					}),
					Next: planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-resource",
						Config: &atc.TaskConfig{},
					}),
				})

				build, err := execEngine.CreateBuild(logger, build, plan)

				Expect(err).NotTo(HaveOccurred())

				build.Resume(logger)

				Expect(inputStep.RunCallCount()).To(Equal(1))

				Expect(taskStep.RunCallCount()).To(Equal(1))
			})
		})
	})
})

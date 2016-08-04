package pipelines_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/pipelines/pipelinesfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/fake_runner"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
)

var _ = Describe("Pipelines Syncer", func() {
	var (
		syncherDB             *pipelinesfakes.FakeSyncherDB
		pipelineDB            *dbfakes.FakePipelineDB
		otherPipelineDB       *dbfakes.FakePipelineDB
		pipelineDBFactory     *dbfakes.FakePipelineDBFactory
		pipelineRunnerFactory PipelineRunnerFactory

		fakeRunner         *fake_runner.FakeRunner
		fakeRunnerExitChan chan error
		otherFakeRunner    *fake_runner.FakeRunner

		syncer *Syncer
	)

	BeforeEach(func() {
		syncherDB = new(pipelinesfakes.FakeSyncherDB)
		pipelineDB = new(dbfakes.FakePipelineDB)

		pipelineDBFactory = new(dbfakes.FakePipelineDBFactory)

		fakeRunner = new(fake_runner.FakeRunner)
		otherFakeRunner = new(fake_runner.FakeRunner)

		pipelineRunnerFactory = func(pipelineDBArg db.PipelineDB) ifrit.Runner {
			switch pipelineDBArg {
			case pipelineDB:
				return fakeRunner
			case otherPipelineDB:
				return otherFakeRunner
			default:
				panic("unexpected pipelineDB input received")
			}
		}

		pipelineDBFactory.BuildStub = func(pipeline db.SavedPipeline) db.PipelineDB {
			switch pipeline.Name {
			case "pipeline":
				return pipelineDB
			case "other-pipeline":
				return otherPipelineDB
			case "renamed-pipeline":
				return pipelineDB
			default:
				panic("unexpected pipeline input received")
			}
		}

		fakeRunnerExitChan = make(chan error, 1)

		// avoid data race
		exitChan := fakeRunnerExitChan

		fakeRunner.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			return <-exitChan
		}

		syncherDB.GetAllPipelinesReturns([]db.SavedPipeline{
			{
				ID: 1,
				Pipeline: db.Pipeline{
					Name: "pipeline",
				},
			},
			{
				ID: 2,
				Pipeline: db.Pipeline{
					Name: "other-pipeline",
				},
			},
		}, nil)

		syncer = NewSyncer(
			lagertest.NewTestLogger("test"),

			syncherDB,
			pipelineDBFactory,
			pipelineRunnerFactory,
		)
	})

	JustBeforeEach(func() {
		syncer.Sync()
	})

	It("spawns a new process for each pipeline", func() {
		Expect(fakeRunner.RunCallCount()).To(Equal(1))
		Expect(otherFakeRunner.RunCallCount()).To(Equal(1))
	})

	Context("when we sync again", func() {
		It("does not spawn any processes again", func() {
			syncer.Sync()
			Expect(fakeRunner.RunCallCount()).To(Equal(1))
		})
	})

	Context("when a pipeline is deleted", func() {
		It("stops the process", func() {
			Expect(fakeRunner.RunCallCount()).To(Equal(1))
			Expect(otherFakeRunner.RunCallCount()).To(Equal(1))

			syncherDB.GetAllPipelinesReturns([]db.SavedPipeline{
				{
					ID: 2,
					Pipeline: db.Pipeline{
						Name: "other-pipeline",
					},
				},
			}, nil)

			syncer.Sync()

			Expect(fakeRunner.RunCallCount()).To(Equal(1))

			signals, _ := fakeRunner.RunArgsForCall(0)
			Eventually(signals).Should(Receive(Equal(os.Interrupt)))
		})

		Context("when another is configured with the same name", func() {
			It("stops the process", func() {
				Expect(fakeRunner.RunCallCount()).To(Equal(1))
				Expect(otherFakeRunner.RunCallCount()).To(Equal(1))

				syncherDB.GetAllPipelinesReturns([]db.SavedPipeline{
					{
						ID: 2,
						Pipeline: db.Pipeline{
							Name: "other-pipeline",
						},
					},
					{
						ID: 3,
						Pipeline: db.Pipeline{
							Name: "pipeline",
						},
					},
				}, nil)

				syncer.Sync()

				Expect(fakeRunner.RunCallCount()).To(Equal(2))

				signals, _ := fakeRunner.RunArgsForCall(0)
				Eventually(signals).Should(Receive(Equal(os.Interrupt)))
			})
		})

		Context("when pipeline name was changed", func() {
			It("recreates syncer with new name", func() {
				Expect(fakeRunner.RunCallCount()).To(Equal(1))
				Expect(otherFakeRunner.RunCallCount()).To(Equal(1))

				syncherDB.GetAllPipelinesReturns([]db.SavedPipeline{
					{
						ID: 1,
						Pipeline: db.Pipeline{
							Name: "renamed-pipeline",
						},
					},
					{
						ID: 2,
						Pipeline: db.Pipeline{
							Name: "other-pipeline",
						},
					},
				}, nil)

				syncer.Sync()

				Expect(fakeRunner.RunCallCount()).To(Equal(2))

				signals, _ := fakeRunner.RunArgsForCall(0)
				Eventually(signals).Should(Receive(Equal(os.Interrupt)))
			})
		})
	})

	Context("when a pipeline is paused", func() {
		pipelines := []db.SavedPipeline{
			{
				ID:     1,
				Paused: true,
				Pipeline: db.Pipeline{
					Name: "pipeline",
				},
			},
			{
				ID: 2,
				Pipeline: db.Pipeline{
					Name: "other-pipeline",
				},
			},
		}

		JustBeforeEach(func() {
			Expect(fakeRunner.RunCallCount()).To(Equal(1))
			Expect(otherFakeRunner.RunCallCount()).To(Equal(1))

			syncherDB.GetAllPipelinesReturns(pipelines, nil)

			syncer.Sync()
		})

		It("stops the process", func() {
			Expect(fakeRunner.RunCallCount()).To(Equal(1))

			signals, _ := fakeRunner.RunArgsForCall(0)
			Eventually(signals).Should(Receive(Equal(os.Interrupt)))
		})
	})

	Context("when the pipeline's process exits", func() {
		BeforeEach(func() {
			fakeRunnerExitChan <- nil
		})

		Context("when we sync again", func() {
			It("spawns the process again", func() {
				Expect(fakeRunner.RunCallCount()).To(Equal(1))
				Expect(otherFakeRunner.RunCallCount()).To(Equal(1))

				fakeRunnerExitChan <- errors.New("disaster")
				syncer.Sync()

				Expect(fakeRunner.RunCallCount()).To(Equal(2))
			})
		})
	})

	Context("when the call to lookup pipelines errors", func() {
		It("does not spawn any processes", func() {
		})
	})
})

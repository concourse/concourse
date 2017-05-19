package pipelines_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/pipelines"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/fake_runner"
)

var _ = Describe("Pipelines Syncer", func() {
	var (
		pipeline              *dbngfakes.FakePipeline
		otherPipeline         *dbngfakes.FakePipeline
		pipeline1             *dbngfakes.FakePipeline
		pipeline2             *dbngfakes.FakePipeline
		pipelineFactory       *dbngfakes.FakePipelineFactory
		pipelineRunnerFactory PipelineRunnerFactory

		fakeRunner         *fake_runner.FakeRunner
		fakeRunnerExitChan chan error
		otherFakeRunner    *fake_runner.FakeRunner

		syncer *Syncer
	)

	BeforeEach(func() {
		pipeline = new(dbngfakes.FakePipeline)
		otherPipeline = new(dbngfakes.FakePipeline)
		pipeline.IDReturns(1)
		otherPipeline.IDReturns(2)

		pipelineFactory = new(dbngfakes.FakePipelineFactory)

		fakeRunner = new(fake_runner.FakeRunner)
		otherFakeRunner = new(fake_runner.FakeRunner)

		pipelineRunnerFactory = func(pipelineArg dbng.Pipeline) ifrit.Runner {
			switch pipelineArg {
			case pipeline:
				return fakeRunner
			case otherPipeline:
				return otherFakeRunner
			default:
				panic("unexpected pipelineDB input received")
			}
		}

		pipelineFactory.GetPipelineByIDStub = func(teamID int, pipelineID int) dbng.Pipeline {
			switch pipelineID {
			case 1:
				return pipeline
			case 2:
				return otherPipeline
			case 3:
				return pipeline
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

		pipeline1 = new(dbngfakes.FakePipeline)
		pipeline2 = new(dbngfakes.FakePipeline)
		pipeline1.IDReturns(1)
		pipeline1.NameReturns("pipeline")
		pipeline2.IDReturns(2)
		pipeline2.NameReturns("other-pipeline")
		pipelineFactory.AllPipelinesReturns([]dbng.Pipeline{pipeline1, pipeline2}, nil)

		syncer = NewSyncer(
			lagertest.NewTestLogger("test"),
			pipelineFactory,
			pipelineRunnerFactory,
		)
	})

	JustBeforeEach(func() {
		syncer.Sync()
	})

	It("spawns a new process for each pipeline", func() {
		Eventually(fakeRunner.RunCallCount).Should(Equal(1))
		Eventually(otherFakeRunner.RunCallCount).Should(Equal(1))
	})

	Context("when we sync again", func() {
		It("does not spawn any processes again", func() {
			syncer.Sync()
			Consistently(fakeRunner.RunCallCount).Should(Equal(1))
		})
	})

	Context("when a pipeline is deleted", func() {
		It("stops the process", func() {
			Eventually(fakeRunner.RunCallCount).Should(Equal(1))
			Eventually(otherFakeRunner.RunCallCount).Should(Equal(1))

			pipelineFactory.AllPipelinesReturns([]dbng.Pipeline{pipeline2}, nil)

			syncer.Sync()

			signals, _ := fakeRunner.RunArgsForCall(0)
			Eventually(signals).Should(Receive(Equal(os.Interrupt)))
		})

		Context("when another is configured with the same name", func() {
			It("stops the process", func() {
				Eventually(fakeRunner.RunCallCount).Should(Equal(1))
				Eventually(otherFakeRunner.RunCallCount).Should(Equal(1))

				pipeline3 := new(dbngfakes.FakePipeline)
				pipeline3.IDReturns(3)
				pipeline3.NameReturns("pipeline")

				pipelineFactory.AllPipelinesReturns([]dbng.Pipeline{pipeline2, pipeline3}, nil)

				syncer.Sync()

				Eventually(fakeRunner.RunCallCount).Should(Equal(2))

				signals, _ := fakeRunner.RunArgsForCall(0)
				Eventually(signals).Should(Receive(Equal(os.Interrupt)))
			})
		})

		Context("when pipeline name was changed", func() {
			It("recreates syncer with new name", func() {
				Eventually(fakeRunner.RunCallCount).Should(Equal(1))
				Eventually(otherFakeRunner.RunCallCount).Should(Equal(1))

				pipeline1.NameReturns("renamed-pipeline")

				pipelineFactory.AllPipelinesReturns([]dbng.Pipeline{pipeline1, pipeline2}, nil)

				syncer.Sync()

				Eventually(fakeRunner.RunCallCount).Should(Equal(2))

				signals, _ := fakeRunner.RunArgsForCall(0)
				Eventually(signals).Should(Receive(Equal(os.Interrupt)))
			})
		})
	})

	Context("when a pipeline is paused", func() {
		JustBeforeEach(func() {
			Eventually(fakeRunner.RunCallCount).Should(Equal(1))
			Eventually(otherFakeRunner.RunCallCount).Should(Equal(1))

			pipeline1.PausedReturns(true)
			pipelineFactory.AllPipelinesReturns([]dbng.Pipeline{pipeline1, pipeline2}, nil)

			syncer.Sync()
		})

		It("stops the process", func() {
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
				Eventually(fakeRunner.RunCallCount).Should(Equal(1))
				Eventually(otherFakeRunner.RunCallCount).Should(Equal(1))

				fakeRunnerExitChan <- errors.New("disaster")
				syncer.Sync()

				Eventually(fakeRunner.RunCallCount).Should(Equal(2))
			})
		})
	})

	Context("when the call to lookup pipelines errors", func() {
		It("does not spawn any processes", func() {
		})
	})
})

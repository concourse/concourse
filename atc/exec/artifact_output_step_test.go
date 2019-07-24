package exec_test

import (
	"context"
	"errors"
	"io/ioutil"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArtifactOutputStep", func() {
	var (
		ctx    context.Context
		cancel func()

		state    exec.RunState
		delegate *execfakes.FakeBuildStepDelegate

		step             exec.Step
		stepErr          error
		plan             atc.Plan
		fakeBuild        *dbfakes.FakeBuild
		fakeWorkerClient *workerfakes.FakeClient
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		state = exec.NewRunState()

		delegate = new(execfakes.FakeBuildStepDelegate)
		delegate.StdoutReturns(ioutil.Discard)

		fakeBuild = new(dbfakes.FakeBuild)
		fakeWorkerClient = new(workerfakes.FakeClient)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan = atc.Plan{ArtifactOutput: &atc.ArtifactOutputPlan{"some-name"}}

		step = exec.NewArtifactOutputStep(plan, fakeBuild, fakeWorkerClient, delegate)
		stepErr = step.Run(ctx, state)
	})

	Context("when the source does not exist", func() {
		It("returns the error", func() {
			Expect(stepErr).To(HaveOccurred())
		})
	})

	Context("when the source exists", func() {
		Context("when the source is not a worker.Volume", func() {
			BeforeEach(func() {
				fakeSource := new(workerfakes.FakeArtifactSource)
				state.ArtifactRepository().RegisterSource(build.ArtifactName("some-name"), fakeSource)
			})
			It("returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
			})
		})

		Context("when the source is a worker.Volume", func() {
			var fakeWorkerVolume *workerfakes.FakeVolume

			BeforeEach(func() {
				fakeWorkerVolume = new(workerfakes.FakeVolume)
				fakeWorkerVolume.HandleReturns("handle")

				source := exec.NewTaskArtifactSource(fakeWorkerVolume)
				state.ArtifactRepository().RegisterSource(build.ArtifactName("some-name"), source)
			})

			Context("when initializing the artifact fails", func() {
				BeforeEach(func() {
					fakeWorkerVolume.InitializeArtifactReturns(nil, errors.New("nope"))
				})
				It("returns the error", func() {
					Expect(stepErr).To(HaveOccurred())
				})
			})

			Context("when initializing the artifact succeeds", func() {
				var fakeWorkerArtifact *dbfakes.FakeWorkerArtifact

				BeforeEach(func() {
					fakeWorkerArtifact = new(dbfakes.FakeWorkerArtifact)
					fakeWorkerArtifact.IDReturns(0)

					fakeWorkerVolume.InitializeArtifactReturns(fakeWorkerArtifact, nil)
				})
				It("succeeds", func() {
					Expect(step.Succeeded()).To(BeTrue())
				})
			})
		})
	})
})

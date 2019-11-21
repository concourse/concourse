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
	"github.com/concourse/concourse/atc/runtime/runtimefakes"
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

		artifactName string
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		state = exec.NewRunState()

		delegate = new(execfakes.FakeBuildStepDelegate)
		delegate.StdoutReturns(ioutil.Discard)

		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.TeamIDReturns(4)

		fakeWorkerClient = new(workerfakes.FakeClient)

		artifactName = "some-artifact-name"
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan = atc.Plan{ArtifactOutput: &atc.ArtifactOutputPlan{artifactName}}

		step = exec.NewArtifactOutputStep(plan, fakeBuild, fakeWorkerClient, delegate)
		stepErr = step.Run(ctx, state)
	})

	Context("when the source does not exist", func() {
		It("returns the error", func() {
			Expect(stepErr).To(HaveOccurred())
		})
	})

	Context("when the artifact exists", func() {
		Context("when the source is not a worker.Volume", func() {
			BeforeEach(func() {
				fakeArtifact := new(runtimefakes.FakeArtifact)
				state.ArtifactRepository().RegisterArtifact(build.ArtifactName(artifactName), fakeArtifact)
			})
			It("returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
			})
		})

		Context("when the source is a worker.Volume", func() {
			var fakeWorkerVolume *workerfakes.FakeVolume
			var fakeArtifact *runtimefakes.FakeArtifact

			BeforeEach(func() {
				fakeWorkerVolume = new(workerfakes.FakeVolume)
				fakeWorkerVolume.HandleReturns("some-volume-handle")

				fakeArtifact = new(runtimefakes.FakeArtifact)
				fakeArtifact.IDReturns("some-artifact-id")

				fakeWorkerClient.FindVolumeReturns(fakeWorkerVolume, true, nil)

				state.ArtifactRepository().RegisterArtifact(build.ArtifactName(artifactName), fakeArtifact)
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

				It("calls workerClient -> FindVolume with the correct arguments", func() {
					_, actualTeamId, actualBuildArtifactID := fakeWorkerClient.FindVolumeArgsForCall(0)
					Expect(actualTeamId).To(Equal(4))
					Expect(actualBuildArtifactID).To(Equal("some-artifact-id"))
				})

				It("succeeds", func() {
					Expect(step.Succeeded()).To(BeTrue())
				})
			})
		})
	})
})

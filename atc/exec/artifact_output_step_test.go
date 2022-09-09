package exec_test

import (
	"context"
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArtifactOutputStep", func() {
	var (
		ctx    context.Context
		cancel func()

		state exec.RunState

		step           exec.Step
		stepOk         bool
		stepErr        error
		plan           atc.Plan
		fakeBuild      *dbfakes.FakeBuild
		fakeWorkerPool *execfakes.FakePool

		artifactName string
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		state = exec.NewRunState(noopStepper, vars.StaticVariables{}, false)

		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.TeamIDReturns(4)

		fakeWorkerPool = new(execfakes.FakePool)

		artifactName = "some-artifact-name"
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan = atc.Plan{ArtifactOutput: &atc.ArtifactOutputPlan{Name: artifactName}}

		step = exec.NewArtifactOutputStep(plan, fakeBuild, fakeWorkerPool)
		stepOk, stepErr = step.Run(ctx, state)
	})

	Context("when the source does not exist", func() {
		It("returns the error", func() {
			Expect(stepErr).To(HaveOccurred())
		})
	})

	Context("when the artifact exists", func() {
		var volume *runtimetest.Volume

		BeforeEach(func() {
			volume = runtimetest.NewVolume("some-volume")
			fakeWorkerPool.LocateVolumeReturns(volume, runtimetest.NewWorker("worker"), true, nil)

			state.ArtifactRepository().RegisterArtifact(build.ArtifactName(artifactName), volume, false)
		})

		Context("when initializing the artifact fails", func() {
			BeforeEach(func() {
				volume.DBVolume_.InitializeArtifactReturns(nil, errors.New("nope"))
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

				volume.DBVolume_.InitializeArtifactReturns(fakeWorkerArtifact, nil)
			})

			It("succeeds", func() {
				Expect(stepOk).To(BeTrue())
			})
		})
	})
})

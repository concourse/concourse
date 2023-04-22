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

var _ = Describe("ArtifactInputStep", func() {
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
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		state = exec.NewRunState(noopStepper, vars.StaticVariables{}, false)

		fakeBuild = new(dbfakes.FakeBuild)
		fakeWorkerPool = new(execfakes.FakePool)

		plan = atc.Plan{ArtifactInput: &atc.ArtifactInputPlan{34, "some-input-artifact-name"}}
		step = exec.NewArtifactInputStep(plan, fakeBuild, fakeWorkerPool)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		stepOk, stepErr = step.Run(ctx, state)
	})

	Context("when looking up the build artifact errors", func() {
		BeforeEach(func() {
			fakeBuild.ArtifactReturns(nil, errors.New("nope"))
		})
		It("returns the error", func() {
			Expect(stepErr).To(HaveOccurred())
		})
	})

	Context("when looking up the build artifact succeeds", func() {
		var fakeWorkerArtifact *dbfakes.FakeWorkerArtifact

		BeforeEach(func() {
			fakeWorkerArtifact = new(dbfakes.FakeWorkerArtifact)
			fakeBuild.ArtifactReturns(fakeWorkerArtifact, nil)
		})

		Context("when looking up the db volume fails", func() {
			BeforeEach(func() {
				fakeWorkerArtifact.VolumeReturns(nil, false, errors.New("nope"))
			})
			It("returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
			})
		})

		Context("when the db volume does not exist", func() {
			BeforeEach(func() {
				fakeWorkerArtifact.VolumeReturns(nil, false, nil)
			})
			It("returns the error", func() {
				Expect(stepErr).To(HaveOccurred())
			})
		})

		Context("when the db volume does exist", func() {
			var fakeVolume *dbfakes.FakeCreatedVolume

			BeforeEach(func() {
				fakeVolume = new(dbfakes.FakeCreatedVolume)
				fakeWorkerArtifact.VolumeReturns(fakeVolume, true, nil)
			})

			Context("when looking up the worker volume fails", func() {
				BeforeEach(func() {
					fakeWorkerPool.LocateVolumeReturns(nil, nil, false, errors.New("nope"))
				})
				It("returns the error", func() {
					Expect(stepErr).To(HaveOccurred())
				})
			})

			Context("when the worker volume does not exist", func() {
				BeforeEach(func() {
					fakeWorkerPool.LocateVolumeReturns(nil, nil, false, nil)
				})
				It("returns an error", func() {
					Expect(stepErr).To(HaveOccurred())
				})
			})

			Context("when the volume does exist", func() {
				var volume *runtimetest.Volume
				var fakeDBWorkerArtifact *dbfakes.FakeWorkerArtifact

				BeforeEach(func() {
					volume = runtimetest.NewVolume("some-volume")
					fakeWorkerPool.LocateVolumeReturns(volume, runtimetest.NewWorker("worker"), true, nil)

					fakeDBWorkerArtifact = new(dbfakes.FakeWorkerArtifact)
					fakeDBWorkerArtifact.VolumeReturns(volume.DBVolume(), true, nil)
					fakeBuild.ArtifactReturns(fakeDBWorkerArtifact, nil)
				})

				It("registers the artifact", func() {
					artifact, fromCache, found := state.ArtifactRepository().ArtifactFor(build.ArtifactName("some-input-artifact-name"))

					Expect(stepErr).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(artifact).To(Equal(volume))
					Expect(fromCache).To(BeFalse())
				})

				It("succeeds", func() {
					Expect(stepOk).To(BeTrue())
				})
			})
		})
	})
})

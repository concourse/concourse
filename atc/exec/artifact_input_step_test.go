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

var _ = Describe("ArtifactInputStep", func() {
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

		plan = atc.Plan{ArtifactInput: &atc.ArtifactInputPlan{34, "some-input-artifact-name"}}
		step = exec.NewArtifactInputStep(plan, fakeBuild, fakeWorkerClient, delegate)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		stepErr = step.Run(ctx, state)
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
					fakeWorkerClient.FindVolumeReturns(nil, false, errors.New("nope"))
				})
				It("returns the error", func() {
					Expect(stepErr).To(HaveOccurred())
				})
			})

			Context("when the worker volume does not exist", func() {
				BeforeEach(func() {
					fakeWorkerClient.FindVolumeReturns(nil, false, nil)
				})
				It("returns the error", func() {
					Expect(stepErr).To(HaveOccurred())
				})
			})

			Context("when the volume does exist", func() {
				var fakeWorkerVolume *workerfakes.FakeVolume
				var fakeDBWorkerArtifact *dbfakes.FakeWorkerArtifact
				var fakeDBCreatedVolume *dbfakes.FakeCreatedVolume

				BeforeEach(func() {
					fakeWorkerVolume = new(workerfakes.FakeVolume)
					fakeWorkerClient.FindVolumeReturns(fakeWorkerVolume, true, nil)

					fakeDBWorkerArtifact = new(dbfakes.FakeWorkerArtifact)
					fakeDBCreatedVolume = new(dbfakes.FakeCreatedVolume)
					fakeDBCreatedVolume.HandleReturns("some-volume-handle")
					fakeDBWorkerArtifact.VolumeReturns(fakeDBCreatedVolume, true, nil)
					fakeBuild.ArtifactReturns(fakeDBWorkerArtifact, nil)
				})

				It("registers the artifact", func() {
					artifact, found := state.ArtifactRepository().ArtifactFor(build.ArtifactName("some-input-artifact-name"))

					Expect(stepErr).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(artifact.ID()).To(Equal("some-volume-handle"))
				})

				It("succeeds", func() {
					Expect(step.Succeeded()).To(BeTrue())
				})
			})
		})
	})
})

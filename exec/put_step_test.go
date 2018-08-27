package exec_test

import (
	"context"
	"errors"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/resourcefakes"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("PutStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeBuild *dbfakes.FakeBuild

		pipelineResourceName string

		fakeResourceFactory *resourcefakes.FakeResourceFactory
		variables           creds.Variables

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		containerMetadata = db.ContainerMetadata{
			Type:     db.ContainerTypePut,
			StepName: "some-step",
		}
		planID       atc.PlanID
		fakeDelegate *execfakes.FakePutDelegate

		resourceTypes creds.VersionedResourceTypes

		repo  *worker.ArtifactRepository
		state *execfakes.FakeRunState

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		putStep *exec.PutStep
		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(42)
		fakeBuild.TeamIDReturns(123)

		planID = atc.PlanID("some-plan-id")

		pipelineResourceName = "some-resource"

		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		variables = template.StaticVariables{
			"custom-param": "source",
			"source-param": "super-secret-source",
		}

		fakeDelegate = new(execfakes.FakePutDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		resourceTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "((custom-param))"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		})

		stepErr = nil
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		putStep = exec.NewPutStep(
			fakeBuild,
			"some-name",
			"some-resource-type",
			pipelineResourceName,
			creds.NewSource(variables, atc.Source{"some": "((source-param))"}),
			creds.NewParams(variables, atc.Params{"some-param": "some-value"}),
			[]string{"some", "tags"},
			fakeDelegate,
			fakeResourceFactory,
			planID,
			containerMetadata,
			stepMetadata,
			resourceTypes,
		)

		stepErr = putStep.Run(ctx, state)
	})

	Context("when repo contains sources", func() {
		var (
			fakeSource        *workerfakes.FakeArtifactSource
			fakeOtherSource   *workerfakes.FakeArtifactSource
			fakeMountedSource *workerfakes.FakeArtifactSource
		)

		BeforeEach(func() {
			fakeSource = new(workerfakes.FakeArtifactSource)
			fakeOtherSource = new(workerfakes.FakeArtifactSource)
			fakeMountedSource = new(workerfakes.FakeArtifactSource)

			repo.RegisterSource("some-source", fakeSource)
			repo.RegisterSource("some-other-source", fakeOtherSource)
			repo.RegisterSource("some-mounted-source", fakeMountedSource)
		})

		Context("when the tracker can initialize the resource", func() {
			var (
				fakeResource        *resourcefakes.FakeResource
				fakeVersionedSource *resourcefakes.FakeVersionedSource
			)

			BeforeEach(func() {
				fakeResource = new(resourcefakes.FakeResource)
				fakeResourceFactory.NewResourceReturns(fakeResource, nil)

				fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
				fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
				fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

				fakeResource.PutReturns(fakeVersionedSource, nil)
			})

			It("initializes the resource with the correct type, session, and sources", func() {
				Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))

				_, _, owner, cm, containerSpec, actualResourceTypes, delegate := fakeResourceFactory.NewResourceArgsForCall(0)
				Expect(cm).To(Equal(containerMetadata))
				Expect(owner).To(Equal(db.NewBuildStepContainerOwner(42, atc.PlanID(planID))))
				Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
					ResourceType: "some-resource-type",
				}))
				Expect(containerSpec.Tags).To(Equal([]string{"some", "tags"}))
				Expect(containerSpec.TeamID).To(Equal(123))
				Expect(containerSpec.Env).To(Equal([]string{"a=1", "b=2"}))
				Expect(containerSpec.Dir).To(Equal("/tmp/build/put"))
				Expect(containerSpec.Inputs).To(HaveLen(3))
				Expect([]worker.ArtifactSource{
					containerSpec.Inputs[0].Source(),
					containerSpec.Inputs[1].Source(),
					containerSpec.Inputs[2].Source(),
				}).To(ConsistOf(
					exec.PutResourceSource{fakeSource},
					exec.PutResourceSource{fakeOtherSource},
					exec.PutResourceSource{fakeMountedSource},
				))
				Expect(actualResourceTypes).To(Equal(resourceTypes))
				Expect(delegate).To(Equal(fakeDelegate))
			})

			It("puts the resource with the given context", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))
				putCtx, _, _, _ := fakeResource.PutArgsForCall(0)
				Expect(putCtx).To(Equal(ctx))
			})

			It("puts the resource with the correct source and params", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))

				_, _, putSource, putParams := fakeResource.PutArgsForCall(0)
				Expect(putSource).To(Equal(atc.Source{"some": "super-secret-source"}))
				Expect(putParams).To(Equal(atc.Params{"some-param": "some-value"}))
			})

			It("puts the resource with the io config forwarded", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))

				_, ioConfig, _, _ := fakeResource.PutArgsForCall(0)
				Expect(ioConfig.Stdout).To(Equal(stdoutBuf))
				Expect(ioConfig.Stderr).To(Equal(stderrBuf))
			})

			It("runs the get resource action", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))
			})

			It("reports the created version info", func() {
				info := putStep.VersionInfo()
				Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
				Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("is successful", func() {
				Expect(putStep.Succeeded()).To(BeTrue())
			})

			It("saves the build output", func() {
				Expect(fakeBuild.SaveOutputCallCount()).To(Equal(1))

				vr := fakeBuild.SaveOutputArgsForCall(0)
				Expect(vr).To(Equal(db.VersionedResource{
					Resource: "some-resource",
					Type:     "some-resource-type",
					Version:  db.ResourceVersion{"some": "version"},
					Metadata: db.NewResourceMetadataFields([]atc.MetadataField{{"some", "metadata"}}),
				}))
			})

			Context("when the resource is blank", func() {
				BeforeEach(func() {
					pipelineResourceName = ""
				})

				It("is successful", func() {
					Expect(putStep.Succeeded()).To(BeTrue())
				})

				It("does not save the build output", func() {
					Expect(fakeBuild.SaveOutputCallCount()).To(Equal(0))
				})
			})

			It("finishes via the delegate", func() {
				Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
				_, status, info := fakeDelegate.FinishedArgsForCall(0)
				Expect(status).To(Equal(exec.ExitStatus(0)))
				Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
				Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("stores the version info as the step result", func() {
				Expect(state.StoreResultCallCount()).To(Equal(1))
				sID, sVal := state.StoreResultArgsForCall(0)
				Expect(sID).To(Equal(planID))
				Expect(sVal).To(Equal(exec.VersionInfo{
					Version:  atc.Version{"some": "version"},
					Metadata: []atc.MetadataField{{"some", "metadata"}},
				}))
			})

			Context("when saving the build output fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeBuild.SaveOutputReturns(disaster)
				})

				It("returns the error", func() {
					Expect(stepErr).To(Equal(disaster))
				})
			})

			Context("when performing the put exits unsuccessfully", func() {
				BeforeEach(func() {
					fakeResource.PutReturns(nil, resource.ErrResourceScriptFailed{
						ExitStatus: 42,
					})
				})

				It("finishes the step via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
					_, status, info := fakeDelegate.FinishedArgsForCall(0)
					Expect(status).To(Equal(exec.ExitStatus(42)))
					Expect(info).To(BeZero())
				})

				It("returns nil", func() {
					Expect(stepErr).ToNot(HaveOccurred())
				})

				It("is not successful", func() {
					Expect(putStep.Succeeded()).To(BeFalse())
				})
			})

			Context("when performing the put errors", func() {
				disaster := errors.New("oh no")

				BeforeEach(func() {
					fakeResource.PutReturns(nil, disaster)
				})

				It("does not finish the step via the delegate", func() {
					Expect(fakeDelegate.FinishedCallCount()).To(Equal(0))
				})

				It("returns the error", func() {
					Expect(stepErr).To(Equal(disaster))
				})

				It("is not successful", func() {
					Expect(putStep.Succeeded()).To(BeFalse())
				})
			})
		})

		Context("when the resource factory fails to create the put resource", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeResourceFactory.NewResourceReturns(nil, disaster)
			})

			It("returns the failure", func() {
				Expect(stepErr).To(Equal(disaster))
			})
		})
	})
})

package exec_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager/lagertest"

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
	"github.com/tedsuo/ifrit"
)

var _ = Describe("PutAction", func() {
	var (
		fakeWorkerClient           *workerfakes.FakeClient
		fakeResourceFactory        *resourcefakes.FakeResourceFactory
		fakeDBResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		variables                  creds.Variables

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		containerMetadata = db.ContainerMetadata{
			Type:     db.ContainerTypePut,
			StepName: "some-step",
		}
		teamID                  = 123
		buildID                 = 42
		planID                  = 56
		fakeBuildEventsDelegate *execfakes.FakeActionsBuildEventsDelegate
		fakeBuildStepDelegate   *execfakes.FakeBuildStepDelegate

		resourceTypes creds.VersionedResourceTypes

		artifactRepository *worker.ArtifactRepository

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		putAction  *exec.PutAction
		actionStep exec.Step
		process    ifrit.Process
	)

	BeforeEach(func() {
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeDBResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		variables = template.StaticVariables{
			"custom-param": "source",
			"source-param": "super-secret-source",
		}

		fakeBuildEventsDelegate = new(execfakes.FakeActionsBuildEventsDelegate)
		fakeBuildStepDelegate = new(execfakes.FakeBuildStepDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeBuildStepDelegate.StdoutReturns(stdoutBuf)
		fakeBuildStepDelegate.StderrReturns(stderrBuf)

		artifactRepository = worker.NewArtifactRepository()

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
	})

	JustBeforeEach(func() {
		putAction = exec.NewPutAction(
			"some-resource-type",
			"some-resource",
			"some-resource",
			creds.NewSource(variables, atc.Source{"some": "((source-param))"}),
			creds.NewParams(variables, atc.Params{"some-param": "some-value"}),
			[]string{"some", "tags"},
			fakeBuildStepDelegate,
			fakeResourceFactory,
			teamID,
			buildID,
			atc.PlanID(planID),
			containerMetadata,
			stepMetadata,
			resourceTypes,
		)

		actionStep = exec.NewActionsStep(
			lagertest.NewTestLogger("put-action-test"),
			[]exec.Action{putAction},
			fakeBuildEventsDelegate,
		).Using(artifactRepository)

		process = ifrit.Invoke(actionStep)
	})

	Context("when artifactRepository contains sources", func() {
		var (
			fakeSource        *workerfakes.FakeArtifactSource
			fakeOtherSource   *workerfakes.FakeArtifactSource
			fakeMountedSource *workerfakes.FakeArtifactSource
		)

		BeforeEach(func() {
			fakeSource = new(workerfakes.FakeArtifactSource)
			fakeOtherSource = new(workerfakes.FakeArtifactSource)
			fakeMountedSource = new(workerfakes.FakeArtifactSource)

			artifactRepository.RegisterSource("some-source", fakeSource)
			artifactRepository.RegisterSource("some-other-source", fakeOtherSource)
			artifactRepository.RegisterSource("some-mounted-source", fakeMountedSource)
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
				Expect(delegate).To(Equal(fakeBuildStepDelegate))
			})

			It("puts the resource with the correct source and params", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))

				_, putSource, putParams, _, _ := fakeResource.PutArgsForCall(0)
				Expect(putSource).To(Equal(atc.Source{"some": "super-secret-source"}))
				Expect(putParams).To(Equal(atc.Params{"some-param": "some-value"}))
			})

			It("puts the resource with the io config forwarded", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))

				ioConfig, _, _, _, _ := fakeResource.PutArgsForCall(0)
				Expect(ioConfig.Stdout).To(Equal(stdoutBuf))
				Expect(ioConfig.Stderr).To(Equal(stderrBuf))
			})

			It("runs the get resource action", func() {
				Expect(fakeResource.PutCallCount()).To(Equal(1))
			})

			It("artifactRepositoryrts the created version info", func() {
				info := putAction.VersionInfo()
				Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
				Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("is successful", func() {
				Expect(putAction.ExitStatus()).To(Equal(exec.ExitStatus(0)))
			})

			Describe("signalling", func() {
				var receivedSignals <-chan os.Signal

				BeforeEach(func() {
					sigs := make(chan os.Signal)
					receivedSignals = sigs

					fakeResource.PutStub = func(
						ioConfig resource.IOConfig,
						source atc.Source,
						params atc.Params,
						signals <-chan os.Signal,
						ready chan<- struct{},
					) (resource.VersionedSource, error) {
						close(ready)
						sigs <- <-signals
						return fakeVersionedSource, nil
					}
				})

				It("forwards to the resource", func() {
					process.Signal(os.Interrupt)
					Eventually(receivedSignals).Should(Receive(Equal(os.Interrupt)))
					Eventually(process.Wait()).Should(Receive())
				})
			})

			Context("when performing the put fails", func() {
				Context("with an unknown error", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeResource.PutReturns(nil, disaster)
					})

					It("exits with the failure", func() {
						Eventually(process.Wait()).Should(Receive(Equal(disaster)))
					})
				})

				Context("by being interrupted", func() {
					BeforeEach(func() {
						fakeResource.PutReturns(nil, resource.ErrAborted)
					})

					It("exits with ErrInterrupted", func() {
						Expect(<-process.Wait()).To(Equal(exec.ErrInterrupted))
					})
				})
			})
		})

		Context("when the resource factory fails to create the put resource", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeResourceFactory.NewResourceReturns(nil, disaster)
			})

			It("exits with the failure", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})
		})
	})
})

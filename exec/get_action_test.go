package exec_test

import (
	"archive/tar"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/creds/credsfakes"
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

var _ = Describe("GetAction", func() {
	var (
		fakeWorkerClient           *workerfakes.FakeClient
		fakeResourceFetcher        *resourcefakes.FakeFetcher
		fakeDBResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeBuildStepDelegate      *execfakes.FakeBuildStepDelegate
		fakeBuildEventsDelegate    *execfakes.FakeActionsBuildEventsDelegate
		fakeVariablesFactory       *credsfakes.FakeVariablesFactory
		variables                  creds.Variables
		fakeBuild                  *dbfakes.FakeBuild

		fakeVersionedSource *resourcefakes.FakeVersionedSource
		resourceTypes       atc.VersionedResourceTypes

		artifactRepository *worker.ArtifactRepository

		factory exec.Factory
		getStep exec.Step
		process ifrit.Process

		containerMetadata = db.ContainerMetadata{
			PipelineID: 4567,
			Type:       db.ContainerTypeGet,
			StepName:   "some-step",
		}

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		teamID  = 123
		buildID = 42
		planID  = 56
	)

	BeforeEach(func() {
		fakeBuildStepDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeBuildEventsDelegate = new(execfakes.FakeActionsBuildEventsDelegate)
		fakeResourceFetcher = new(resourcefakes.FakeFetcher)
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeDBResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)

		fakeVariablesFactory = new(credsfakes.FakeVariablesFactory)
		variables = template.StaticVariables{
			"source-param": "super-secret-source",
		}
		fakeVariablesFactory.NewVariablesReturns(variables)

		artifactRepository = worker.NewArtifactRepository()
		fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
		fakeResourceFetcher.FetchReturns(fakeVersionedSource, nil)

		fakeResourceFactory := new(resourcefakes.FakeResourceFactory)

		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(buildID)
		fakeBuild.TeamIDReturns(teamID)

		resourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "source"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		factory = exec.NewGardenFactory(fakeWorkerClient, fakeResourceFetcher, fakeResourceFactory, fakeDBResourceCacheFactory, fakeVariablesFactory)
	})

	JustBeforeEach(func() {
		getStep = factory.Get(
			lagertest.NewTestLogger("get-action-test"),
			atc.Plan{
				ID: atc.PlanID(planID),
				Get: &atc.GetPlan{
					Type:                   "some-resource-type",
					Name:                   "some-resource",
					Source:                 atc.Source{"some": "((source-param))"},
					Params:                 atc.Params{"some-param": "some-value"},
					Tags:                   []string{"some", "tags"},
					Version:                &atc.Version{"some-version": "some-value"},
					VersionedResourceTypes: resourceTypes,
				},
			},
			fakeBuild,
			stepMetadata,
			containerMetadata,
			fakeBuildEventsDelegate,
			fakeBuildStepDelegate,
		).Using(artifactRepository)

		process = ifrit.Invoke(getStep)
	})

	It("initializes the resource with the correct type and session id, making sure that it is not ephemeral", func() {
		Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(1))
		_, sid, tags, actualTeamID, actualResourceTypes, resourceInstance, sm, delegate, _, _ := fakeResourceFetcher.FetchArgsForCall(0)
		Expect(sm).To(Equal(stepMetadata))
		Expect(sid).To(Equal(resource.Session{
			Metadata: db.ContainerMetadata{
				PipelineID:       4567,
				Type:             db.ContainerTypeGet,
				StepName:         "some-step",
				WorkingDirectory: "/tmp/build/get",
			},
		}))
		Expect(tags).To(ConsistOf("some", "tags"))
		Expect(actualTeamID).To(Equal(teamID))
		Expect(resourceInstance).To(Equal(resource.NewResourceInstance(
			"some-resource-type",
			atc.Version{"some-version": "some-value"},
			atc.Source{"some": "super-secret-source"},
			atc.Params{"some-param": "some-value"},
			creds.NewVersionedResourceTypes(variables, resourceTypes),
			nil,
			db.NewBuildStepContainerOwner(buildID, atc.PlanID(planID)),
		)))
		Expect(actualResourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, resourceTypes)))
		Expect(delegate).To(Equal(fakeBuildStepDelegate))
		expectedLockName := fmt.Sprintf("%x",
			sha256.Sum256([]byte(
				`{"type":"some-resource-type","version":{"some-version":"some-value"},"source":{"some":"super-secret-source"},"params":{"some-param":"some-value"},"worker_name":"fake-worker"}`,
			)),
		)

		Expect(resourceInstance.LockName("fake-worker")).To(Equal(expectedLockName))
	})

	Context("when fetching resource succeeds", func() {
		BeforeEach(func() {
			fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
			fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})
		})

		Describe("the source registered with the repository", func() {
			var artifactSource worker.ArtifactSource

			JustBeforeEach(func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				var found bool
				artifactSource, found = artifactRepository.SourceFor("some-resource")
				Expect(found).To(BeTrue())
			})

			Describe("streaming to a destination", func() {
				var fakeDestination *workerfakes.FakeArtifactDestination

				BeforeEach(func() {
					fakeDestination = new(workerfakes.FakeArtifactDestination)
				})

				Context("when the resource can stream out", func() {
					var (
						streamedOut io.ReadCloser
					)

					BeforeEach(func() {
						streamedOut = gbytes.NewBuffer()
						fakeVersionedSource.StreamOutReturns(streamedOut, nil)
					})

					It("streams the resource to the destination", func() {
						err := artifactSource.StreamTo(fakeDestination)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeVersionedSource.StreamOutCallCount()).To(Equal(1))
						Expect(fakeVersionedSource.StreamOutArgsForCall(0)).To(Equal("."))

						Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
						dest, src := fakeDestination.StreamInArgsForCall(0)
						Expect(dest).To(Equal("."))
						Expect(src).To(Equal(streamedOut))
					})

					Context("when streaming out of the versioned source fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVersionedSource.StreamOutReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(artifactSource.StreamTo(fakeDestination)).To(Equal(disaster))
						})
					})

					Context("when streaming in to the destination fails", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeDestination.StreamInReturns(disaster)
						})

						It("returns the error", func() {
							Expect(artifactSource.StreamTo(fakeDestination)).To(Equal(disaster))
						})
					})
				})

				Context("when the resource cannot stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.StreamOutReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(artifactSource.StreamTo(fakeDestination)).To(Equal(disaster))
					})
				})
			})

			Describe("streaming a file out", func() {
				Context("when the resource can stream out", func() {
					var (
						fileContent = "file-content"

						tarBuffer *gbytes.Buffer
					)

					BeforeEach(func() {
						tarBuffer = gbytes.NewBuffer()
						fakeVersionedSource.StreamOutReturns(tarBuffer, nil)
					})

					Context("when the file exists", func() {
						BeforeEach(func() {
							tarWriter := tar.NewWriter(tarBuffer)

							err := tarWriter.WriteHeader(&tar.Header{
								Name: "some-file",
								Mode: 0644,
								Size: int64(len(fileContent)),
							})
							Expect(err).NotTo(HaveOccurred())

							_, err = tarWriter.Write([]byte(fileContent))
							Expect(err).NotTo(HaveOccurred())
						})

						It("streams out the given path", func() {
							reader, err := artifactSource.StreamFile("some-path")
							Expect(err).NotTo(HaveOccurred())

							Expect(ioutil.ReadAll(reader)).To(Equal([]byte(fileContent)))

							Expect(fakeVersionedSource.StreamOutArgsForCall(0)).To(Equal("some-path"))
						})

						Describe("closing the stream", func() {
							It("closes the stream from the versioned source", func() {
								reader, err := artifactSource.StreamFile("some-path")
								Expect(err).NotTo(HaveOccurred())

								Expect(tarBuffer.Closed()).To(BeFalse())

								err = reader.Close()
								Expect(err).NotTo(HaveOccurred())

								Expect(tarBuffer.Closed()).To(BeTrue())
							})
						})
					})

					Context("but the stream is empty", func() {
						It("returns ErrFileNotFound", func() {
							_, err := artifactSource.StreamFile("some-path")
							Expect(err).To(MatchError(exec.FileNotFoundError{Path: "some-path"}))
						})
					})
				})

				Context("when the resource cannot stream out", func() {
					disaster := errors.New("nope")

					BeforeEach(func() {
						fakeVersionedSource.StreamOutReturns(nil, disaster)
					})

					It("returns the error", func() {
						_, err := artifactSource.StreamFile("some-path")
						Expect(err).To(Equal(disaster))
					})
				})
			})
		})
	})
})

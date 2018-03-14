package exec_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
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
)

var _ = Describe("GetStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeWorkerClient           *workerfakes.FakeClient
		fakeResourceFetcher        *resourcefakes.FakeFetcher
		fakeDBResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeVariablesFactory       *credsfakes.FakeVariablesFactory
		variables                  creds.Variables
		fakeBuild                  *dbfakes.FakeBuild
		fakeDelegate               *execfakes.FakeGetDelegate
		getPlan                    *atc.GetPlan

		fakeVersionedSource *resourcefakes.FakeVersionedSource
		resourceTypes       atc.VersionedResourceTypes

		artifactRepository *worker.ArtifactRepository
		state              *execfakes.FakeRunState

		factory exec.Factory
		getStep exec.Step
		stepErr error

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
		ctx, cancel = context.WithCancel(context.Background())

		fakeResourceFetcher = new(resourcefakes.FakeFetcher)
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeDBResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)

		fakeVariablesFactory = new(credsfakes.FakeVariablesFactory)
		variables = template.StaticVariables{
			"source-param": "super-secret-source",
		}
		fakeVariablesFactory.NewVariablesReturns(variables)

		artifactRepository = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(artifactRepository)

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

		getPlan = &atc.GetPlan{
			Name:                   "some-name",
			Type:                   "some-resource-type",
			Source:                 atc.Source{"some": "((source-param))"},
			Params:                 atc.Params{"some-param": "some-value"},
			Tags:                   []string{"some", "tags"},
			Version:                &atc.Version{"some-version": "some-value"},
			VersionedResourceTypes: resourceTypes,
		}

		factory = exec.NewGardenFactory(fakeWorkerClient, fakeResourceFetcher, fakeResourceFactory, fakeDBResourceCacheFactory, fakeVariablesFactory)

		fakeDelegate = new(execfakes.FakeGetDelegate)
	})

	JustBeforeEach(func() {
		getStep = factory.Get(
			lagertest.NewTestLogger("get-action-test"),
			atc.Plan{
				ID:  atc.PlanID(planID),
				Get: getPlan,
			},
			fakeBuild,
			stepMetadata,
			containerMetadata,
			fakeDelegate,
		)

		stepErr = getStep.Run(ctx, state)
	})

	It("initializes the resource with the correct type and session id, making sure that it is not ephemeral", func() {
		Expect(stepErr).ToNot(HaveOccurred())

		Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(1))
		fctx, _, sid, tags, actualTeamID, actualResourceTypes, resourceInstance, sm, delegate := fakeResourceFetcher.FetchArgsForCall(0)
		Expect(fctx).To(Equal(ctx))
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
		Expect(delegate).To(Equal(fakeDelegate))
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

		It("returns nil", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})

		It("is successful", func() {
			Expect(getStep.Succeeded()).To(BeTrue())
		})

		It("finishes the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, status, info := fakeDelegate.FinishedArgsForCall(0)
			Expect(status).To(Equal(exec.ExitStatus(0)))
			Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
			Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
		})

		Context("when getting a pipeline resource", func() {
			BeforeEach(func() {
				getPlan.Resource = "some-pipeline-resource"
			})

			It("saves the build input so that the metadata is visible", func() {
				// TODO: this can be removed once /check returns metadata

				Expect(fakeBuild.SaveInputCallCount()).To(Equal(1))

				input := fakeBuild.SaveInputArgsForCall(0)
				Expect(input).To(Equal(db.BuildInput{
					Name: "some-name",
					VersionedResource: db.VersionedResource{
						Resource: "some-pipeline-resource",
						Type:     "some-resource-type",
						Version:  db.ResourceVersion{"some": "version"},
						Metadata: db.NewResourceMetadataFields([]atc.MetadataField{{"some", "metadata"}}),
					},
				}))
			})
		})

		Context("when getting an anonymous resource", func() {
			BeforeEach(func() {
				getPlan.Resource = ""
			})

			It("does not save the build input", func() {
				// TODO: this can be removed once /check returns metadata

				Expect(fakeBuild.SaveInputCallCount()).To(Equal(0))
			})
		})

		Describe("the source registered with the repository", func() {
			var artifactSource worker.ArtifactSource

			JustBeforeEach(func() {
				var found bool
				artifactSource, found = artifactRepository.SourceFor("some-name")
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

						tgzBuffer *gbytes.Buffer
					)

					BeforeEach(func() {
						tgzBuffer = gbytes.NewBuffer()
						fakeVersionedSource.StreamOutReturns(tgzBuffer, nil)
					})

					Context("when the file exists", func() {
						BeforeEach(func() {
							gzWriter := gzip.NewWriter(tgzBuffer)
							defer gzWriter.Close()

							tarWriter := tar.NewWriter(gzWriter)
							defer tarWriter.Close()

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

								Expect(tgzBuffer.Closed()).To(BeFalse())

								err = reader.Close()
								Expect(err).NotTo(HaveOccurred())

								Expect(tgzBuffer.Closed()).To(BeTrue())
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

	Context("when fetching the resource exits unsuccessfully", func() {
		BeforeEach(func() {
			fakeResourceFetcher.FetchReturns(nil, resource.ErrResourceScriptFailed{
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
			Expect(getStep.Succeeded()).To(BeFalse())
		})
	})

	Context("when fetching the resource errors", func() {
		disaster := errors.New("oh no")

		BeforeEach(func() {
			fakeResourceFetcher.FetchReturns(nil, disaster)
		})

		It("does not finish the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(0))
		})

		It("returns the error", func() {
			Expect(stepErr).To(Equal(disaster))
		})

		It("is not successful", func() {
			Expect(getStep.Succeeded()).To(BeFalse())
		})
	})
})

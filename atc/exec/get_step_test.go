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
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("GetStep", func() {
	var (
		ctx        context.Context
		cancel     func()
		testLogger *lagertest.TestLogger

		fakeWorker               *workerfakes.FakeWorker
		fakePool                 *workerfakes.FakePool
		fakeStrategy             *workerfakes.FakeContainerPlacementStrategy
		fakeResourceFetcher      *resourcefakes.FakeFetcher
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeVariablesFactory     *credsfakes.FakeVariablesFactory
		variables                creds.Variables
		fakeBuild                *dbfakes.FakeBuild
		fakeDelegate             *execfakes.FakeGetDelegate
		getPlan                  *atc.GetPlan

		fakeVersionedSource *resourcefakes.FakeVersionedSource
		resourceTypes       atc.VersionedResourceTypes

		artifactRepository *artifact.Repository
		state              *execfakes.FakeRunState

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
		testLogger = lagertest.NewTestLogger("get-action-test")
		ctx, cancel = context.WithCancel(context.Background())

		fakeWorker = new(workerfakes.FakeWorker)
		fakeResourceFetcher = new(resourcefakes.FakeFetcher)
		fakePool = new(workerfakes.FakePool)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)

		fakeVariablesFactory = new(credsfakes.FakeVariablesFactory)
		variables = template.StaticVariables{
			"source-param": "super-secret-source",
		}
		fakeVariablesFactory.NewVariablesReturns(variables)

		artifactRepository = artifact.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(artifactRepository)

		fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
		fakeResourceFetcher.FetchReturns(fakeVersionedSource, nil)

		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(buildID)
		fakeBuild.TeamIDReturns(teamID)
		fakeBuild.PipelineNameReturns("pipeline")

		fakeDelegate = new(execfakes.FakeGetDelegate)

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

		containerMetadata.WorkingDirectory = resource.ResourcesDir("get")
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:  atc.PlanID(planID),
			Get: getPlan,
		}

		variables := fakeVariablesFactory.NewVariables(fakeBuild.TeamName(), fakeBuild.PipelineName())

		getStep = exec.NewGetStep(
			fakeBuild,

			plan.Get.Name,
			plan.Get.Type,
			plan.Get.Resource,
			creds.NewSource(variables, plan.Get.Source),
			creds.NewParams(variables, plan.Get.Params),
			exec.NewVersionSourceFromPlan(plan.Get),
			plan.Get.Tags,

			fakeDelegate,
			fakeResourceFetcher,
			fakeBuild.TeamID(),
			fakeBuild.ID(),
			plan.ID,
			containerMetadata,
			fakeResourceCacheFactory,
			stepMetadata,

			creds.NewVersionedResourceTypes(variables, plan.Get.VersionedResourceTypes),

			fakeStrategy,
			fakePool,
		)

		stepErr = getStep.Run(ctx, state)
	})

	It("finds or chooses a worker", func() {
		Expect(fakePool.FindOrChooseWorkerForContainerCallCount()).To(Equal(1))
		_, actualOwner, actualContainerSpec, actualWorkerSpec, strategy := fakePool.FindOrChooseWorkerForContainerArgsForCall(0)
		Expect(actualOwner).To(Equal(db.NewBuildStepContainerOwner(buildID, atc.PlanID(planID), teamID)))
		Expect(actualContainerSpec).To(Equal(worker.ContainerSpec{
			ImageSpec: worker.ImageSpec{
				ResourceType: "some-resource-type",
			},
			TeamID: teamID,
			Env:    stepMetadata.Env(),
		}))
		Expect(actualWorkerSpec).To(Equal(worker.WorkerSpec{
			ResourceType:  "some-resource-type",
			Tags:          atc.Tags{"some", "tags"},
			TeamID:        teamID,
			ResourceTypes: creds.NewVersionedResourceTypes(variables, resourceTypes),
		}))
		Expect(strategy).To(Equal(fakeStrategy))
	})

	Context("when find or choosing worker succeeds", func() {
		BeforeEach(func() {
			fakeWorker.NameReturns("some-worker")
			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)
		})

		It("initializes the resource with the correct type and session id, making sure that it is not ephemeral", func() {
			Expect(stepErr).ToNot(HaveOccurred())

			Expect(fakeResourceFetcher.FetchCallCount()).To(Equal(1))
			fctx, _, sid, actualWorker, actualContainerSpec, actualResourceTypes, resourceInstance, delegate := fakeResourceFetcher.FetchArgsForCall(0)
			Expect(fctx).To(Equal(ctx))
			Expect(sid).To(Equal(resource.Session{
				Metadata: db.ContainerMetadata{
					PipelineID:       4567,
					Type:             db.ContainerTypeGet,
					StepName:         "some-step",
					WorkingDirectory: "/tmp/build/get",
				},
			}))
			Expect(actualWorker.Name()).To(Equal("some-worker"))
			Expect(actualContainerSpec).To(Equal(worker.ContainerSpec{
				ImageSpec: worker.ImageSpec{
					ResourceType: "some-resource-type",
				},
				TeamID: teamID,
				Env:    stepMetadata.Env(),
			}))
			Expect(resourceInstance).To(Equal(resource.NewResourceInstance(
				"some-resource-type",
				atc.Version{"some-version": "some-value"},
				atc.Source{"some": "super-secret-source"},
				atc.Params{"some-param": "some-value"},
				creds.NewVersionedResourceTypes(variables, resourceTypes),
				nil,
				db.NewBuildStepContainerOwner(buildID, atc.PlanID(planID), teamID),
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
				var fakeResourceCache *dbfakes.FakeUsedResourceCache
				var fakeResourceConfig *dbfakes.FakeResourceConfig

				BeforeEach(func() {
					getPlan.Resource = "some-pipeline-resource"

					fakeResourceCache = new(dbfakes.FakeUsedResourceCache)
					fakeResourceConfig = new(dbfakes.FakeResourceConfig)
					fakeResourceCache.ResourceConfigReturns(fakeResourceConfig)
					fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeResourceCache, nil)
				})

				It("finds the pipeline", func() {
					Expect(fakeBuild.PipelineCallCount()).To(Equal(1))
				})

				Context("when finding the pipeline succeeds", func() {
					var fakePipeline *dbfakes.FakePipeline

					BeforeEach(func() {
						fakePipeline = new(dbfakes.FakePipeline)
						fakeBuild.PipelineReturns(fakePipeline, true, nil)
					})

					It("finds the resource", func() {
						Expect(fakePipeline.ResourceCallCount()).To(Equal(1))

						Expect(fakePipeline.ResourceArgsForCall(0)).To(Equal(getPlan.Resource))
					})

					Context("when finding the resource succeeds", func() {
						var fakeResource *dbfakes.FakeResource

						BeforeEach(func() {
							fakeResource = new(dbfakes.FakeResource)
							fakePipeline.ResourceReturns(fakeResource, true, nil)
						})

						It("saves the resource config version", func() {
							Expect(fakeResource.SaveUncheckedVersionCallCount()).To(Equal(1))

							version, metadata, resourceConfig, actualResourceTypes := fakeResource.SaveUncheckedVersionArgsForCall(0)
							Expect(version).To(Equal(atc.Version{"some": "version"}))
							Expect(metadata).To(Equal(db.NewResourceConfigMetadataFields([]atc.MetadataField{{"some", "metadata"}})))
							Expect(resourceConfig).To(Equal(fakeResourceConfig))
							Expect(actualResourceTypes).To(Equal(creds.NewVersionedResourceTypes(variables, resourceTypes)))
						})

						Context("when it fails to save the version", func() {
							disaster := errors.New("oops")

							BeforeEach(func() {
								fakeResource.SaveUncheckedVersionReturns(false, disaster)
							})

							It("returns an error", func() {
								Expect(stepErr).To(Equal(disaster))
							})
						})
					})

					Context("when it fails to find the resource", func() {
						disaster := errors.New("oops")

						BeforeEach(func() {
							fakePipeline.ResourceReturns(nil, false, disaster)
						})

						It("returns an error", func() {
							Expect(stepErr).To(Equal(disaster))
						})
					})

					Context("when the resource is not found", func() {
						BeforeEach(func() {
							fakePipeline.ResourceReturns(nil, false, nil)
						})

						It("returns an ErrResourceNotFound", func() {
							Expect(stepErr).To(Equal(exec.ErrResourceNotFound{"some-pipeline-resource"}))
						})
					})
				})

				Context("when it fails to find the pipeline", func() {
					disaster := errors.New("oops")

					BeforeEach(func() {
						fakeBuild.PipelineReturns(nil, false, disaster)
					})

					It("returns an error", func() {
						Expect(stepErr).To(Equal(disaster))
					})
				})

				Context("when the pipeline is not found", func() {
					BeforeEach(func() {
						fakeBuild.PipelineReturns(nil, false, nil)
					})

					It("returns an ErrPipelineNotFound", func() {
						Expect(stepErr).To(Equal(exec.ErrPipelineNotFound{"pipeline"}))
					})
				})
			})

			Context("when getting an anonymous resource", func() {
				var fakeResourceCache *dbfakes.FakeUsedResourceCache
				var fakeResourceConfig *dbfakes.FakeResourceConfig
				BeforeEach(func() {
					getPlan.Resource = ""

					fakeResourceCache = new(dbfakes.FakeUsedResourceCache)
					fakeResourceConfig = new(dbfakes.FakeResourceConfig)
					fakeResourceCache.ResourceConfigReturns(fakeResourceConfig)
					fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeResourceCache, nil)
				})

				It("does not find the pipeline", func() {
					// TODO: this can be removed once /check returns metadata
					Expect(fakeBuild.PipelineCallCount()).To(Equal(0))
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
							err := artifactSource.StreamTo(testLogger, fakeDestination)
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
								Expect(artifactSource.StreamTo(testLogger, fakeDestination)).To(Equal(disaster))
							})
						})

						Context("when streaming in to the destination fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeDestination.StreamInReturns(disaster)
							})

							It("returns the error", func() {
								Expect(artifactSource.StreamTo(testLogger, fakeDestination)).To(Equal(disaster))
							})
						})
					})

					Context("when the resource cannot stream out", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVersionedSource.StreamOutReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(artifactSource.StreamTo(testLogger, fakeDestination)).To(Equal(disaster))
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
								reader, err := artifactSource.StreamFile(testLogger, "some-path")
								Expect(err).NotTo(HaveOccurred())

								Expect(ioutil.ReadAll(reader)).To(Equal([]byte(fileContent)))

								Expect(fakeVersionedSource.StreamOutArgsForCall(0)).To(Equal("some-path"))
							})

							Describe("closing the stream", func() {
								It("closes the stream from the versioned source", func() {
									reader, err := artifactSource.StreamFile(testLogger, "some-path")
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
								_, err := artifactSource.StreamFile(testLogger, "some-path")
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
							_, err := artifactSource.StreamFile(testLogger, "some-path")
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

	Context("when finding or choosing the worker exits unsuccessfully", func() {
		disaster := errors.New("oh no")

		BeforeEach(func() {
			fakePool.FindOrChooseWorkerForContainerReturns(nil, disaster)
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

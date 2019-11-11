package exec_test

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/DataDog/zstd"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/resource/resourcefakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	"github.com/concourse/concourse/vars"
)

var _ = Describe("GetStep", func() {
	var (
		ctx        context.Context
		cancel     func()
		testLogger *lagertest.TestLogger
		stdoutBuf  *gbytes.Buffer
		stderrBuf  *gbytes.Buffer

		fakeClient   *workerfakes.FakeClient
		fakeWorker   *workerfakes.FakeWorker
		fakePool     *workerfakes.FakePool
		fakeStrategy *workerfakes.FakeContainerPlacementStrategy
		fakeFetcher  *workerfakes.FakeFetcher

		fakeResourceFactory      *resourcefakes.FakeResourceFactory
		fakeResource             *resourcefakes.FakeResource
		fakeResourceCacheFactory *dbfakes.FakeResourceCacheFactory
		fakeResourceCache        *dbfakes.FakeUsedResourceCache

		fakeDelegate *execfakes.FakeGetDelegate

		getPlan *atc.GetPlan

		// fakeVersionedSource       *resourcefakes.FakeVersionedSource
		interpolatedResourceTypes atc.VersionedResourceTypes

		artifactRepository *build.Repository
		fakeState          *execfakes.FakeRunState

		getStep exec.Step
		stepErr error

		credVarsTracker vars.CredVarsTracker

		containerMetadata = db.ContainerMetadata{
			WorkingDirectory: resource.ResourcesDir("get"),
			PipelineID:       4567,
			Type:             db.ContainerTypeGet,
			StepName:         "some-step",
		}

		stepMetadata = exec.StepMetadata{
			TeamID:       123,
			TeamName:     "some-team",
			BuildID:      42,
			BuildName:    "some-build",
			PipelineID:   4567,
			PipelineName: "some-pipeline",
		}

		planID = 56
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("get-action-test")
		ctx, cancel = context.WithCancel(context.Background())

		fakeClient = new(workerfakes.FakeClient)
		fakeWorker = new(workerfakes.FakeWorker)
		fakeWorker.NameReturns("some-worker")
		fakeFetcher = new(workerfakes.FakeFetcher)
		fakePool = new(workerfakes.FakePool)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeResource = new(resourcefakes.FakeResource)
		fakeResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeResourceCache = new(dbfakes.FakeUsedResourceCache)

		credVars := vars.StaticVariables{"source-param": "super-secret-source"}
		credVarsTracker = vars.NewCredVarsTracker(credVars, true)

		artifactRepository = build.NewRepository()
		fakeState = new(execfakes.FakeRunState)

		fakeState.ArtifactRepositoryReturns(artifactRepository)

		fakeDelegate = new(execfakes.FakeGetDelegate)
		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
		fakeDelegate.VariablesReturns(credVarsTracker)
		fakeDelegate.StdoutReturns(stdoutBuf)
		fakeDelegate.StderrReturns(stderrBuf)

		uninterpolatedResourceTypes := atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "((source-param))"},
				},
				Version: atc.Version{"some-custom": "version"},
			},
		}

		interpolatedResourceTypes = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "super-secret-source"},
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
			VersionedResourceTypes: uninterpolatedResourceTypes,
		}
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		plan := atc.Plan{
			ID:  atc.PlanID(planID),
			Get: getPlan,
		}

		fakeResourceCacheFactory.FindOrCreateResourceCacheReturns(fakeResourceCache, nil)
		fakeResourceFactory.NewResourceReturns(fakeResource)

		getStep = exec.NewGetStep(
			plan.ID,
			*plan.Get,
			stepMetadata,
			containerMetadata,
			fakeResourceFactory,
			fakeResourceCacheFactory,
			fakeStrategy,
			fakePool,
			fakeDelegate,
			fakeClient,
		)

		stepErr = getStep.Run(ctx, fakeState)
	})

	It("calls RunGetStep with the correct ctx, logger", func() {
		actualCtx, actualLogger, _, _, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualCtx).To(Equal(ctx))
		Expect(actualLogger).To(Equal(testLogger))
	})

	It("calls RunGetStep with the correct ContainerOwner", func() {
		_, _, actualContainerOwner, _, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualContainerOwner).To(Equal(db.NewBuildStepContainerOwner(
			stepMetadata.BuildID,
			atc.PlanID(planID),
			stepMetadata.TeamID,
		)))
	})

	It("calls RunGetStep with the correct ContainerSpec", func() {
		_, _, _, actualContainerSpec, _, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualContainerSpec).To(Equal(
			worker.ContainerSpec{
				ImageSpec: worker.ImageSpec{
					ResourceType: "some-resource-type",
				},
				TeamID: stepMetadata.TeamID,
				Env:    stepMetadata.Env(),
			},
		))
	})

	It("calls RunGetStep with the correct WorkerSpec", func() {
		_, _, _, _, actualWorkerSpec, _, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualWorkerSpec).To(Equal(
			worker.WorkerSpec{
				ResourceType:  "some-resource-type",
				Tags:          atc.Tags{"some", "tags"},
				TeamID:        stepMetadata.TeamID,
				ResourceTypes: interpolatedResourceTypes,
			},
		))
	})

	It("calls RunGetStep with the correct ContainerPlacementStrategy", func() {
		_, _, _, _, _, actualStrategy, _, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualStrategy).To(Equal(fakeStrategy))
	})

	It("calls RunGetStep with the correct ContainerMetadata", func() {
		_, _, _, _, _, _, actualContainerMetadata, _, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualContainerMetadata).To(Equal(
			db.ContainerMetadata{
				PipelineID:       4567,
				Type:             db.ContainerTypeGet,
				StepName:         "some-step",
				WorkingDirectory: "/tmp/build/get",
			},
		))
	})

	It("calls RunGetStep with the correct ImageFetcherSpec", func() {
		_, _, _, _, _, _, _, actualImageFetcherSpec, _, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualImageFetcherSpec).To(Equal(
			worker.ImageFetcherSpec{
				ResourceTypes: interpolatedResourceTypes,
				Delegate:      fakeDelegate,
			},
		))
	})

	It("calls RunGetStep with the correct ProcessSpec", func() {
		_, _, _, _, _, _, _, _, actualProcessSpec, _, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualProcessSpec).To(Equal(
			runtime.ProcessSpec{
				Path:         "/opt/resource/in",
				Args:         []string{resource.ResourcesDir("get")},
				StdoutWriter: fakeDelegate.Stdout(),
				StderrWriter: fakeDelegate.Stderr(),
			},
		))
	})

	It("calls RunGetStep with the correct ResourceCache", func() {
		_, _, _, _, _, _, _, _, _, actualResourceCache, _ := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualResourceCache).To(Equal(fakeResourceCache))
	})

	It("calls RunGetStep with the correct Resource", func() {
		_, _, _, _, _, _, _, _, _, _, actualResource := fakeClient.RunGetStepArgsForCall(0)
		Expect(actualResource).To(Equal(fakeResource))
	})

	Context("when Client.RunGetStep returns an err", func() {
		var disaster error
		JustBeforeEach(func() {
			disaster = errors.New("disaster")
			fakeClient.RunGetStepReturns(worker.GetResult{}, disaster)
		})
		It("returns an err", func() {
			Expect(fakeClient.RunGetStepCallCount()).To(Equal(1))
			Expect(stepErr).To(HaveOccurred())
			Expect(stepErr).To(Equal(disaster))
		})
	})

	Context("when Client.RunGetStep returns a Successful GetResult", func() {
		JustBeforeEach(func() {
			fakeClient.RunGetStepReturns(
				worker.GetResult{
					Status: 0,
					VersionResult: runtime.VersionResult{
						Version: atc.Version{"some": "version"},
						Metadata: []atc.MetadataField{{Name: "some", Value: "metadata"}},
					},
					GetArtifact: runtime.GetArtifact{VolumeHandle:"some-volume-handle"},
				}, nil)
		})

		It("registers the resulting artifact in the RunState.ArtifactRepository", func() {
			Expect(artifactRepository.ArtifactFor(build.ArtifactName(getPlan.Name))).To(Equal(runtime.GetArtifact{"some-volume-handle"}))
		})

		It("marks the step as succeeded", func() {
			Expect(getStep.Succeeded()).To(BeTrue())
		})

		It("finishes the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, status, info := fakeDelegate.FinishedArgsForCall(0)
			Expect(status).To(Equal(exec.ExitStatus(0)))
			Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
			Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
		})

		Context("when the plan has a resource", func() {
			BeforeEach(func() {
				getPlan.Resource = "some-pipeline-resource"
			})

			It("saves a version for the resource", func() {
				Expect(fakeDelegate.UpdateVersionCallCount()).To(Equal(1))
				_, actualPlan, actualVersionResult := fakeDelegate.UpdateVersionArgsForCall(0)
				Expect(actualPlan.Resource).To(Equal("some-pipeline-resource"))
				Expect(actualVersionResult.Version).To(Equal(atc.Version{"some": "version"}))
				Expect(actualVersionResult.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
			})
		})

		Context("when getting an anonymous resource", func() {
			BeforeEach(func() {
				getPlan.Resource = ""
			})

			It("does not save the version", func() {
				Expect(fakeDelegate.UpdateVersionCallCount()).To(Equal(0))
			})
		})

		It("does not return an err", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})
	})

	Context("when Client.RunGetStep returns a Failed GetResult", func() {
		JustBeforeEach(func() {
			fakeClient.RunGetStepReturns(
				worker.GetResult{
					Status: 1,
					VersionResult: runtime.VersionResult{},
				}, nil)
		})

		It("does NOT mark the step as succeeded", func() {
			Expect(getStep.Succeeded()).To(BeFalse())
		})

		It("finishes the step via the delegate", func() {
			Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
			_, actualExitStatus, actualVersionResult := fakeDelegate.FinishedArgsForCall(0)
			Expect(actualExitStatus).ToNot(Equal(exec.ExitStatus(0)))
			Expect(actualVersionResult).To(Equal(runtime.VersionResult{}))
		})

		It("does not return an err", func() {
			Expect(stepErr).ToNot(HaveOccurred())
		})

	})

	Context("when find or choosing worker succeeds", func() {
		BeforeEach(func() {
			fakeWorker.NameReturns("some-worker")
			fakePool.FindOrChooseWorkerForContainerReturns(fakeWorker, nil)
		})

		// TODO: move this to the fetcher_tests package
		It("initializes the resource with the correct type and session id, making sure that it is not ephemeral", func() {
			Expect(stepErr).ToNot(HaveOccurred())

			Expect(fakeFetcher.FetchCallCount()).To(Equal(1))
			// fctx, _, actualContainerMetadata, actualWorker, actualContainerSpec, actualResourceTypes, resourceInstance, delegate := fakeFetcher.FetchArgsForCall(0)
			fctx, _, actualContainerMetadata, actualWorker, actualContainerSpec, actualProcessSpec, actualResource, actualOwner, actualImageFetcherSpec, actualCache, actualLockName := fakeFetcher.FetchArgsForCall(0)
			Expect(fctx).To(Equal(ctx))
			Expect(actualContainerMetadata).To(Equal(db.ContainerMetadata{
				PipelineID:       4567,
				Type:             db.ContainerTypeGet,
				StepName:         "some-step",
				WorkingDirectory: "/tmp/build/get",
			}))
			Expect(actualWorker.Name()).To(Equal("some-worker"))
			Expect(actualContainerSpec).To(Equal(worker.ContainerSpec{
				ImageSpec: worker.ImageSpec{
					ResourceType: "some-resource-type",
				},
				TeamID: stepMetadata.TeamID,
				Env:    stepMetadata.Env(),
			}))
			Expect(actualProcessSpec).To(Equal(runtime.ProcessSpec{
				Path:         "/opt/resource/in",
				Args:         []string{resource.ResourcesDir("get")},
				StdoutWriter: fakeDelegate.Stdout(),
				StderrWriter: fakeDelegate.Stderr(),
			}))
			Expect(actualImageFetcherSpec).To(Equal(worker.ImageFetcherSpec{
				ResourceTypes: interpolatedResourceTypes,
				Delegate:      fakeDelegate,
			}))
			Expect(actualResource).To(Equal(fakeResource))
			// Expect(resourceInstance).To(Equal(resource.NewResourceInstance(
			// 	"some-resource-type",
			// 	atc.Version{"some-version": "some-value"},
			// 	atc.Source{"some": "super-secret-source"},
			// 	atc.Params{"some-param": "some-value"},
			// 	interpolatedResourceTypes,
			// 	nil,
			// 	db.NewBuildStepContainerOwner(stepMetadata.BuildID, atc.PlanID(planID), stepMetadata.TeamID),
			// )))
			// Expect(actualResourceTypes).To(Equal(interpolatedResourceTypes))
			expectedLockName := fmt.Sprintf("%x",
				sha256.Sum256([]byte(
					`{"type":"some-resource-type","version":{"some-version":"some-value"},"source":{"some":"super-secret-source"},"params":{"some-param":"some-value"},"worker_name":"fake-worker"}`,
				)),
			)

			Expect(actualLockName).To(Equal(expectedLockName))
		})

		It("secrets are tracked", func() {
			mapit := vars.NewMapCredVarsTrackerIterator()
			credVarsTracker.IterateInterpolatedCreds(mapit)
			Expect(mapit.Data["source-param"]).To(Equal("super-secret-source"))
		})

		Context("when fetching resource succeeds", func() {
			XIt("returns nil", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			XIt("is successful", func() {
				Expect(getStep.Succeeded()).To(BeTrue())
			})

			XIt("finishes the step via the delegate", func() {
				Expect(fakeDelegate.FinishedCallCount()).To(Equal(1))
				_, status, info := fakeDelegate.FinishedArgsForCall(0)
				Expect(status).To(Equal(exec.ExitStatus(0)))
				Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
				Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
			})

			XContext("when the plan has a resource", func() {
				BeforeEach(func() {
					getPlan.Resource = "some-pipeline-resource"
				})

				It("saves a version for the resource", func() {
					Expect(fakeDelegate.UpdateVersionCallCount()).To(Equal(1))
					_, plan, info := fakeDelegate.UpdateVersionArgsForCall(0)
					Expect(plan.Resource).To(Equal("some-pipeline-resource"))
					Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
					Expect(info.Metadata).To(Equal([]atc.MetadataField{{Name: "some", Value: "metadata"}}))
				})
			})

			XContext("when getting an anonymous resource", func() {

				BeforeEach(func() {
					getPlan.Resource = ""
				})

				It("does not save the version", func() {
					Expect(fakeDelegate.UpdateVersionCallCount()).To(Equal(0))
				})
			})

			Describe("the source registered with the repository", func() {
				var artifactSource worker.StreamableArtifactSource

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
							err := artifactSource.StreamTo(context.TODO(), testLogger, fakeDestination)
							Expect(err).NotTo(HaveOccurred())

							Expect(fakeVersionedSource.StreamOutCallCount()).To(Equal(1))
							_, path := fakeVersionedSource.StreamOutArgsForCall(0)
							Expect(path).To(Equal("."))

							Expect(fakeDestination.StreamInCallCount()).To(Equal(1))
							_, dest, src := fakeDestination.StreamInArgsForCall(0)
							Expect(dest).To(Equal("."))
							Expect(src).To(Equal(streamedOut))
						})

						Context("when streaming out of the versioned source fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeVersionedSource.StreamOutReturns(nil, disaster)
							})

							It("returns the error", func() {
								Expect(artifactSource.StreamTo(context.TODO(), testLogger, fakeDestination)).To(Equal(disaster))
							})
						})

						Context("when streaming in to the destination fails", func() {
							disaster := errors.New("nope")

							BeforeEach(func() {
								fakeDestination.StreamInReturns(disaster)
							})

							It("returns the error", func() {
								Expect(artifactSource.StreamTo(context.TODO(), testLogger, fakeDestination)).To(Equal(disaster))
							})
						})
					})

					Context("when the resource cannot stream out", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVersionedSource.StreamOutReturns(nil, disaster)
						})

						It("returns the error", func() {
							Expect(artifactSource.StreamTo(context.TODO(), testLogger, fakeDestination)).To(Equal(disaster))
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
								zstdWriter := zstd.NewWriter(tgzBuffer)
								defer zstdWriter.Close()

								tarWriter := tar.NewWriter(zstdWriter)
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
								reader, err := artifactSource.StreamFile(context.TODO(), testLogger, "some-path")
								Expect(err).NotTo(HaveOccurred())

								Expect(ioutil.ReadAll(reader)).To(Equal([]byte(fileContent)))
								_, path := fakeVersionedSource.StreamOutArgsForCall(0)
								Expect(path).To(Equal("some-path"))
							})

							Describe("closing the stream", func() {
								It("closes the stream from the versioned source", func() {
									reader, err := artifactSource.StreamFile(context.TODO(), testLogger, "some-path")
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
								_, err := artifactSource.StreamFile(context.TODO(), testLogger, "some-path")
								Expect(err).To(MatchError(runtime.FileNotFoundError{Path: "some-path"}))
							})
						})
					})

					Context("when the resource cannot stream out", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							fakeVersionedSource.StreamOutReturns(nil, disaster)
						})

						It("returns the error", func() {
							_, err := artifactSource.StreamFile(context.TODO(), testLogger, "some-path")
							Expect(err).To(Equal(disaster))
						})
					})
				})
			})
		})

	})

})

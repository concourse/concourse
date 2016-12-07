package exec_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/exec"
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

var _ = Describe("GardenFactory", func() {
	var (
		fakeWorkerClient           *workerfakes.FakeClient
		fakeResourceFactory        *resourcefakes.FakeResourceFactory
		fakeDBResourceCacheFactory *dbngfakes.FakeResourceCacheFactory

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		stepMetadata testMetadata = []string{"a=1", "b=2"}

		identifier = worker.Identifier{
			ResourceID: 1234,
		}
		workerMetadata = worker.Metadata{
			PipelineName: "some-pipeline",
			Type:         db.ContainerTypePut,
			StepName:     "some-step",
		}
		teamID = 123
	)

	BeforeEach(func() {
		fakeWorkerClient = new(workerfakes.FakeClient)
		fakeResourceFetcher := new(resourcefakes.FakeFetcher)
		fakeResourceFactory = new(resourcefakes.FakeResourceFactory)
		fakeDBResourceCacheFactory = new(dbngfakes.FakeResourceCacheFactory)

		factory = NewGardenFactory(fakeWorkerClient, fakeResourceFetcher, fakeResourceFactory, fakeDBResourceCacheFactory)

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("Put", func() {
		var (
			putDelegate    *execfakes.FakePutDelegate
			resourceConfig atc.ResourceConfig
			params         atc.Params
			tags           []string
			resourceTypes  atc.ResourceTypes

			inStep *execfakes.FakeStep
			repo   *worker.ArtifactRepository

			step    Step
			process ifrit.Process
		)

		BeforeEach(func() {
			putDelegate = new(execfakes.FakePutDelegate)
			putDelegate.StdoutReturns(stdoutBuf)
			putDelegate.StderrReturns(stderrBuf)

			resourceConfig = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-resource-type",
				Source: atc.Source{"some": "source"},
			}

			params = atc.Params{"some-param": "some-value"}
			tags = []string{"some", "tags"}

			inStep = new(execfakes.FakeStep)
			repo = worker.NewArtifactRepository()

			resourceTypes = atc.ResourceTypes{
				{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"some-custom": "source"},
				},
			}
		})

		JustBeforeEach(func() {
			step = factory.Put(
				lagertest.NewTestLogger("test"),
				stepMetadata,
				identifier,
				workerMetadata,
				putDelegate,
				resourceConfig,
				tags,
				teamID,
				params,
				resourceTypes,
			).Using(inStep, repo)

			process = ifrit.Invoke(step)
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
					fakeResourceFactory.NewResourceReturns(fakeResource, []string{"some-source", "some-other-source"}, nil)

					fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
					fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
					fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

					fakeResource.PutReturns(fakeVersionedSource, nil)
				})

				It("initializes the resource with the correct type, session, and sources", func() {
					Expect(fakeResourceFactory.NewResourceCallCount()).To(Equal(1))

					_, sid, sm, resourceSpec, actualResourceTypes, delegate, sources := fakeResourceFactory.NewResourceArgsForCall(0)
					Expect(sm).To(Equal(worker.Metadata{
						PipelineName:     "some-pipeline",
						Type:             db.ContainerTypePut,
						StepName:         "some-step",
						WorkingDirectory: "/tmp/build/put",
					}))
					Expect(sid).To(Equal(worker.Identifier{
						ResourceID: 1234,
						Stage:      db.ContainerStageRun,
					}))
					Expect(resourceSpec).To(Equal(worker.ContainerSpec{
						ImageSpec: worker.ImageSpec{
							ResourceType: "some-resource-type",
							Privileged:   true,
						},
						Ephemeral: true,
						Tags:      []string{"some", "tags"},
						TeamID:    123,
						Env:       []string{"a=1", "b=2"},
					}))
					Expect(actualResourceTypes).To(Equal(atc.ResourceTypes{
						{
							Name:   "custom-resource",
							Type:   "custom-type",
							Source: atc.Source{"some-custom": "source"},
						},
					}))
					Expect(delegate).To(Equal(putDelegate))

					// TODO: Can we test the map values?
					Expect(sources).To(HaveKey("some-source"))
					Expect(sources).To(HaveKey("some-other-source"))
					Expect(sources).To(HaveKey("some-mounted-source"))
				})

				It("puts the resource with the correct source and params, and the full repository as the artifact source", func() {
					Expect(fakeResource.PutCallCount()).To(Equal(1))

					_, putSource, putParams, putArtifactSource, _, _ := fakeResource.PutArgsForCall(0)
					Expect(putSource).To(Equal(resourceConfig.Source))
					Expect(putParams).To(Equal(params))

					dest := new(workerfakes.FakeArtifactDestination)

					err := putArtifactSource.StreamTo(dest)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeSource.StreamToCallCount()).To(Equal(1))

					sourceDest := fakeSource.StreamToArgsForCall(0)
					someStream := new(bytes.Buffer)

					err = sourceDest.StreamIn("foo", someStream)
					Expect(err).NotTo(HaveOccurred())

					Expect(dest.StreamInCallCount()).To(Equal(1))
					destPath, stream := dest.StreamInArgsForCall(0)
					Expect(destPath).To(Equal("some-source/foo"))
					Expect(stream).To(Equal(someStream))

					Expect(fakeOtherSource.StreamToCallCount()).To(Equal(1))

					otherSourceDest := fakeOtherSource.StreamToArgsForCall(0)
					someOtherStream := new(bytes.Buffer)

					err = otherSourceDest.StreamIn("foo", someOtherStream)
					Expect(err).NotTo(HaveOccurred())

					Expect(dest.StreamInCallCount()).To(Equal(2))
					otherDestPath, otherStream := dest.StreamInArgsForCall(1)
					Expect(otherDestPath).To(Equal("some-other-source/foo"))
					Expect(otherStream).To(Equal(someOtherStream))

					Expect(fakeMountedSource.StreamToCallCount()).To(Equal(0))
				})

				It("puts the resource with the io config forwarded", func() {
					Expect(fakeResource.PutCallCount()).To(Equal(1))

					ioConfig, _, _, _, _, _ := fakeResource.PutArgsForCall(0)
					Expect(ioConfig.Stdout).To(Equal(stdoutBuf))
					Expect(ioConfig.Stderr).To(Equal(stderrBuf))
				})

				It("runs the get resource action", func() {
					Expect(fakeResource.PutCallCount()).To(Equal(1))
				})

				It("reports the created version info", func() {
					var info VersionInfo
					Expect(step.Result(&info)).To(BeTrue())
					Expect(info.Version).To(Equal(atc.Version{"some": "version"}))
					Expect(info.Metadata).To(Equal([]atc.MetadataField{{"some", "metadata"}}))
				})

				It("is successful", func() {
					Eventually(process.Wait()).Should(Receive(BeNil()))

					var success Success
					Expect(step.Result(&success)).To(BeTrue())
					Expect(bool(success)).To(BeTrue())
				})

				It("releases the resource", func() {
					<-process.Wait()

					Expect(fakeResource.ReleaseCallCount()).To(BeZero())

					step.Release()
					Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
				})

				It("completes via the delegate", func() {
					Eventually(putDelegate.CompletedCallCount).Should(Equal(1))

					exitStatus, verionInfo := putDelegate.CompletedArgsForCall(0)
					Expect(exitStatus).To(Equal(ExitStatus(0)))
					Expect(verionInfo).To(Equal(&VersionInfo{
						Version:  atc.Version{"some": "version"},
						Metadata: []atc.MetadataField{{"some", "metadata"}},
					}))
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
							artifactSource worker.ArtifactSource,
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

						It("invokes the delegate's Failed callback without completing", func() {
							Eventually(process.Wait()).Should(Receive(Equal(disaster)))

							Expect(putDelegate.CompletedCallCount()).To(BeZero())

							Expect(putDelegate.FailedCallCount()).To(Equal(1))
							Expect(putDelegate.FailedArgsForCall(0)).To(Equal(disaster))
						})

						It("releases the resource", func() {
							<-process.Wait()

							Expect(fakeResource.ReleaseCallCount()).To(BeZero())

							step.Release()
							Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
						})
					})

					Context("by being interrupted", func() {
						BeforeEach(func() {
							fakeResource.PutReturns(nil, resource.ErrAborted)
						})

						It("exits with ErrInterrupted", func() {
							Expect(<-process.Wait()).To(Equal(ErrInterrupted))
						})

						It("invokes the delegate's Failed callback without completing", func() {
							<-process.Wait()

							Expect(putDelegate.CompletedCallCount()).To(BeZero())

							Expect(putDelegate.FailedCallCount()).To(Equal(1))
							Expect(putDelegate.FailedArgsForCall(0)).To(Equal(ErrInterrupted))
						})

						It("releases the resource", func() {
							<-process.Wait()

							Expect(fakeResource.ReleaseCallCount()).To(BeZero())

							step.Release()
							Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
						})
					})

					Context("with a resource script failure", func() {
						var resourceScriptError resource.ErrResourceScriptFailed

						BeforeEach(func() {
							resourceScriptError = resource.ErrResourceScriptFailed{
								ExitStatus: 1,
							}

							fakeResource.PutReturns(nil, resourceScriptError)
						})

						It("invokes the delegate's Finished callback instead of failed", func() {
							Eventually(process.Wait()).Should(Receive())

							Expect(putDelegate.FailedCallCount()).To(BeZero())

							Expect(putDelegate.CompletedCallCount()).To(Equal(1))
							status, versionInfo := putDelegate.CompletedArgsForCall(0)
							Expect(status).To(Equal(ExitStatus(1)))
							Expect(versionInfo).To(BeNil())
						})

						It("is not successful", func() {
							Eventually(process.Wait()).Should(Receive(BeNil()))
							Expect(putDelegate.CompletedCallCount()).To(Equal(1))

							var success Success

							Expect(step.Result(&success)).To(BeTrue())
							Expect(bool(success)).To(BeFalse())
						})

						It("releases the resource", func() {
							<-process.Wait()

							Expect(fakeResource.ReleaseCallCount()).To(BeZero())

							step.Release()
							Expect(fakeResource.ReleaseCallCount()).To(Equal(1))
						})
					})
				})
			})

			Context("when the resource factory fails to create the put resource", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeResourceFactory.NewResourceReturns(nil, nil, disaster)
				})

				It("exits with the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("invokes the delegate's Failed callback", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))

					Expect(putDelegate.CompletedCallCount()).To(BeZero())

					Expect(putDelegate.FailedCallCount()).To(Equal(1))
					Expect(putDelegate.FailedArgsForCall(0)).To(Equal(disaster))
				})
			})
		})

		Context("when there are no sources in repo", func() {
			var (
				fakeResource        *resourcefakes.FakeResource
				fakeVersionedSource *resourcefakes.FakeVersionedSource
			)

			BeforeEach(func() {
				fakeResource = new(resourcefakes.FakeResource)
				fakeResourceFactory.NewResourceReturns(fakeResource, []string{}, nil)

				fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
				fakeResource.PutReturns(fakeVersionedSource, nil)
			})

			It("streams in empty source", func() {
				_, _, _, resourceSource, _, _ := fakeResource.PutArgsForCall(0)
				fakeDestination := new(workerfakes.FakeArtifactDestination)
				resourceSource.StreamTo(fakeDestination)

				Expect(fakeDestination.StreamInCallCount()).To(Equal(1))

				path, reader := fakeDestination.StreamInArgsForCall(0)
				Expect(path).To(Equal("."))

				tarReader := tar.NewReader(reader)

				_, err := tarReader.Next()
				Expect(err).To(Equal(io.EOF))
			})
		})
	})
})

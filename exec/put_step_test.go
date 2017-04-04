package exec_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
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

		workerMetadata = dbng.ContainerMetadata{
			Type:     dbng.ContainerTypePut,
			StepName: "some-step",
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
			resourceTypes  atc.VersionedResourceTypes

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
		})

		JustBeforeEach(func() {
			step = factory.Put(
				lagertest.NewTestLogger("test"),
				teamID,
				42,
				atc.PlanID("some-plan-id"),
				stepMetadata,
				workerMetadata,
				putDelegate,
				resourceConfig,
				tags,
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
					fakeResourceFactory.NewPutResourceReturns(fakeResource, nil)

					fakeVersionedSource = new(resourcefakes.FakeVersionedSource)
					fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
					fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

					fakeResource.PutReturns(fakeVersionedSource, nil)
				})

				It("initializes the resource with the correct type, session, and sources", func() {
					Expect(fakeResourceFactory.NewPutResourceCallCount()).To(Equal(1))

					_, _, buildID, planID, sm, containerSpec, actualResourceTypes, delegate := fakeResourceFactory.NewPutResourceArgsForCall(0)
					Expect(sm).To(Equal(dbng.ContainerMetadata{
						Type:             dbng.ContainerTypePut,
						StepName:         "some-step",
						WorkingDirectory: "/tmp/build/put",
					}))
					Expect(buildID).To(Equal(42))
					Expect(planID).To(Equal(atc.PlanID("some-plan-id")))
					Expect(containerSpec.ImageSpec).To(Equal(worker.ImageSpec{
						ResourceType: "some-resource-type",
						Privileged:   true,
					}))
					Expect(containerSpec.Tags).To(Equal([]string{"some", "tags"}))
					Expect(containerSpec.TeamID).To(Equal(123))
					Expect(containerSpec.Env).To(Equal([]string{"a=1", "b=2"}))
					Expect(containerSpec.Inputs).To(HaveLen(3))
					Expect([]worker.ArtifactName{
						containerSpec.Inputs[0].Name(),
						containerSpec.Inputs[1].Name(),
						containerSpec.Inputs[2].Name(),
					}).To(ConsistOf([]worker.ArtifactName{
						"some-source",
						"some-other-source",
						"some-mounted-source",
					}))
					Expect(actualResourceTypes).To(Equal(resourceTypes))
					Expect(delegate).To(Equal(putDelegate))
				})

				It("puts the resource with the correct source and params", func() {
					Expect(fakeResource.PutCallCount()).To(Equal(1))

					_, putSource, putParams, _, _ := fakeResource.PutArgsForCall(0)
					Expect(putSource).To(Equal(resourceConfig.Source))
					Expect(putParams).To(Equal(params))
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
					})
				})
			})

			Context("when the resource factory fails to create the put resource", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeResourceFactory.NewPutResourceReturns(nil, disaster)
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
	})
})

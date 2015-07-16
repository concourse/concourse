package exec_test

import (
	"bytes"
	"errors"
	"os"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/fakes"
	"github.com/concourse/atc/resource"
	rfakes "github.com/concourse/atc/resource/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("GardenFactory", func() {
	var (
		fakeTracker      *rfakes.FakeTracker
		fakeWorkerClient *wfakes.FakeClient

		factory Factory

		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		identifier = worker.Identifier{
			Name: "some-session-id",
		}
	)

	BeforeEach(func() {
		fakeTracker = new(rfakes.FakeTracker)
		fakeWorkerClient = new(wfakes.FakeClient)

		factory = NewGardenFactory(fakeWorkerClient, fakeTracker, func() string { return "" })

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()
	})

	Describe("Put", func() {
		var (
			putDelegate    *fakes.FakePutDelegate
			resourceConfig atc.ResourceConfig
			params         atc.Params
			tags           []string

			inStep *fakes.FakeStep
			repo   *SourceRepository

			fakeSource *fakes.FakeArtifactSource

			step    Step
			process ifrit.Process
		)

		BeforeEach(func() {
			putDelegate = new(fakes.FakePutDelegate)
			putDelegate.StdoutReturns(stdoutBuf)
			putDelegate.StderrReturns(stderrBuf)

			resourceConfig = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-resource-type",
				Source: atc.Source{"some": "source"},
			}

			params = atc.Params{"some-param": "some-value"}
			tags = []string{"some", "tags"}

			inStep = new(fakes.FakeStep)
			repo = NewSourceRepository()

			fakeSource = new(fakes.FakeArtifactSource)
			repo.RegisterSource("some-source", fakeSource)
		})

		JustBeforeEach(func() {
			step = factory.Put(identifier, putDelegate, resourceConfig, tags, params).Using(inStep, repo)
			process = ifrit.Invoke(step)
		})

		Context("when the tracker can initialize the resource", func() {
			var (
				fakeResource        *rfakes.FakeResource
				fakeVersionedSource *rfakes.FakeVersionedSource
			)

			BeforeEach(func() {
				fakeResource = new(rfakes.FakeResource)
				fakeTracker.InitReturns(fakeResource, nil)

				fakeVersionedSource = new(rfakes.FakeVersionedSource)
				fakeVersionedSource.VersionReturns(atc.Version{"some": "version"})
				fakeVersionedSource.MetadataReturns([]atc.MetadataField{{"some", "metadata"}})

				fakeResource.PutReturns(fakeVersionedSource)
			})

			It("initializes the resource with the correct type and session id", func() {
				Ω(fakeTracker.InitCallCount()).Should(Equal(1))

				sid, typ, tags := fakeTracker.InitArgsForCall(0)
				Ω(sid).Should(Equal(resource.Session{
					ID: identifier,
				}))
				Ω(typ).Should(Equal(resource.ResourceType("some-resource-type")))
				Ω(tags).Should(ConsistOf("some", "tags"))
			})

			It("puts the resource with the correct source and params, and the full repository as the artifact source", func() {
				Ω(fakeResource.PutCallCount()).Should(Equal(1))

				_, putSource, putParams, putArtifactSource := fakeResource.PutArgsForCall(0)
				Ω(putSource).Should(Equal(resourceConfig.Source))
				Ω(putParams).Should(Equal(params))

				dest := new(fakes.FakeArtifactDestination)

				err := putArtifactSource.StreamTo(dest)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeSource.StreamToCallCount()).Should(Equal(1))

				sourceDest := fakeSource.StreamToArgsForCall(0)

				someStream := new(bytes.Buffer)

				err = sourceDest.StreamIn("foo", someStream)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(dest.StreamInCallCount()).Should(Equal(1))
				destPath, stream := dest.StreamInArgsForCall(0)
				Ω(destPath).Should(Equal("some-source/foo"))
				Ω(stream).Should(Equal(someStream))
			})

			It("puts the resource with the io config forwarded", func() {
				Ω(fakeResource.PutCallCount()).Should(Equal(1))

				ioConfig, _, _, _ := fakeResource.PutArgsForCall(0)
				Ω(ioConfig.Stdout).Should(Equal(stdoutBuf))
				Ω(ioConfig.Stderr).Should(Equal(stderrBuf))
			})

			It("runs the get resource action", func() {
				Ω(fakeVersionedSource.RunCallCount()).Should(Equal(1))
			})

			It("reports the created version info", func() {
				var info VersionInfo
				Ω(step.Result(&info)).Should(BeTrue())
				Ω(info.Version).Should(Equal(atc.Version{"some": "version"}))
				Ω(info.Metadata).Should(Equal([]atc.MetadataField{{"some", "metadata"}}))
			})

			It("is successful", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))

				var success Success
				Ω(step.Result(&success)).Should(BeTrue())
				Ω(bool(success)).Should(BeTrue())
			})

			It("completes via the delegate", func() {
				Eventually(putDelegate.CompletedCallCount).Should(Equal(1))

				exitStatus, verionInfo := putDelegate.CompletedArgsForCall(0)
				Ω(exitStatus).Should(Equal(ExitStatus(0)))
				Ω(verionInfo).Should(Equal(&VersionInfo{
					Version:  atc.Version{"some": "version"},
					Metadata: []atc.MetadataField{{"some", "metadata"}},
				}))
			})

			Describe("signalling", func() {
				var receivedSignals <-chan os.Signal

				BeforeEach(func() {
					sigs := make(chan os.Signal)
					receivedSignals = sigs

					fakeVersionedSource.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(ready)
						sigs <- <-signals
						return nil
					}
				})

				It("forwards to the resource", func() {
					process.Signal(os.Interrupt)
					Eventually(receivedSignals).Should(Receive(Equal(os.Interrupt)))
					Eventually(process.Wait()).Should(Receive())
				})
			})

			Context("when fetching fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeVersionedSource.RunReturns(disaster)
				})

				It("exits with the failure", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("invokes the delegate's Failed callback without completing", func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))

					Ω(putDelegate.CompletedCallCount()).Should(BeZero())

					Ω(putDelegate.FailedCallCount()).Should(Equal(1))
					Ω(putDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
				})

				Context("with a resource script failure", func() {
					var resourceScriptError resource.ErrResourceScriptFailed

					BeforeEach(func() {
						resourceScriptError = resource.ErrResourceScriptFailed{
							ExitStatus: 1,
						}

						fakeVersionedSource.RunReturns(resourceScriptError)
					})

					It("invokes the delegate's Finished callback instead of failed", func() {
						Eventually(process.Wait()).Should(Receive())

						Ω(putDelegate.FailedCallCount()).Should(BeZero())

						Ω(putDelegate.CompletedCallCount()).Should(Equal(1))
						status, versionInfo := putDelegate.CompletedArgsForCall(0)
						Ω(status).Should(Equal(ExitStatus(1)))
						Ω(versionInfo).Should(BeNil())
					})

					It("is not successful", func() {
						Eventually(process.Wait()).Should(Receive(BeNil()))
						Ω(putDelegate.CompletedCallCount()).Should(Equal(1))

						var success Success

						Ω(step.Result(&success)).Should(BeTrue())
						Ω(bool(success)).Should(BeFalse())
					})
				})
			})

			Describe("releasing", func() {
				It("releases the resource", func() {
					Ω(fakeResource.ReleaseCallCount()).Should(BeZero())

					err := step.Release()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeResource.ReleaseCallCount()).Should(Equal(1))
				})
			})
		})

		Context("when the tracker fails to initialize the resource", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeTracker.InitReturns(nil, disaster)
			})

			It("exits with the failure", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))
			})

			It("invokes the delegate's Failed callback", func() {
				Eventually(process.Wait()).Should(Receive(Equal(disaster)))

				Ω(putDelegate.CompletedCallCount()).Should(BeZero())

				Ω(putDelegate.FailedCallCount()).Should(Equal(1))
				Ω(putDelegate.FailedArgsForCall(0)).Should(Equal(disaster))
			})

			Describe("releasing", func() {
				JustBeforeEach(func() {
					Eventually(process.Wait()).Should(Receive(Equal(disaster)))
				})

				It("succeeds", func() {
					err := step.Release()
					Ω(err).ShouldNot(HaveOccurred())
				})
			})
		})
	})
})

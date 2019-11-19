package worker_test

import (
	"context"
	"errors"
	"io"
	"io/ioutil"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/gclient/gclientfakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)


var _ = Describe("RunScript", func() {
	var (

		testLogger lager.Logger

		fakeGardenContainerScriptStdout string
		fakeGardenContainerScriptStderr string
		scriptExitStatus                int

		runErr         error
		attachErr      error
		runScriptErr	error

		scriptProcess *gardenfakes.FakeProcess

		stderrBuf *gbytes.Buffer

		fakeGClientContainer *gclientfakes.FakeContainer
		fakeGClient *gclientfakes.FakeClient
		fakeVolumeClient *workerfakes.FakeVolumeClient
		fakeDBVolumeRepository	*dbfakes.FakeVolumeRepository
		fakeImageFactory *workerfakes.FakeImageFactory
		fakeFetcher *workerfakes.FakeFetcher
		fakeDBTeamFactory *dbfakes.FakeTeamFactory
		fakeDBWorker *dbfakes.FakeWorker
		fakeCreatedContainer *dbfakes.FakeCreatedContainer

		gardenWorker worker.Worker
		workerContainer worker.Container
		fakeDelegate *workerfakes.FakeImageFetchingDelegate
		fakeOwner *dbfakes.FakeContainerOwner


		runScriptCtx  context.Context
		runScriptCancel func()

		runScriptBinPath string
		runScriptArgs    []string
		runScriptInput   []byte
		runScriptOutput  map[string]string
		runScriptLogDestination io.Writer
		runScriptRecoverable bool
	)

	BeforeEach(func() {
		testLogger = lager.NewLogger("test-logger")
		fakeDBVolumeRepository = new(dbfakes.FakeVolumeRepository)
		fakeGClientContainer = new(gclientfakes.FakeContainer)
		fakeCreatedContainer = new(dbfakes.FakeCreatedContainer)
		fakeGClient = new(gclientfakes.FakeClient)
		fakeVolumeClient = new(workerfakes.FakeVolumeClient)
		fakeImageFactory = new(workerfakes.FakeImageFactory)
		fakeFetcher = new(workerfakes.FakeFetcher)
		fakeDBTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeDBWorker = new(dbfakes.FakeWorker)

		fakeDelegate = new(workerfakes.FakeImageFetchingDelegate)
		fakeOwner = new(dbfakes.FakeContainerOwner)

		stderrBuf = gbytes.NewBuffer()

		fakeGardenContainerScriptStdout = "{}"
		fakeGardenContainerScriptStderr = ""
		scriptExitStatus = 0

		runErr = nil
		attachErr = nil

		scriptProcess = new(gardenfakes.FakeProcess)
		scriptProcess.IDReturns("some-proc-id")
		scriptProcess.WaitStub = func() (int, error) {
			return scriptExitStatus, nil
		}

		gardenWorker = worker.NewGardenWorker(
			fakeGClient,
			fakeDBVolumeRepository,
			fakeVolumeClient,
			fakeImageFactory,
			fakeFetcher,
			fakeDBTeamFactory,
			fakeDBWorker,
			0,
		)

		fakeCreatedContainer.HandleReturns("some-handle")
		fakeDBWorker.FindContainerReturns(nil, fakeCreatedContainer, nil)
		fakeGClient.LookupReturns(fakeGClientContainer, nil)

		workerContainer, _ = gardenWorker.FindOrCreateContainer(
			context.TODO(),
			testLogger,
			fakeDelegate,
			fakeOwner,
			db.ContainerMetadata{},
			worker.ContainerSpec{},
			atc.VersionedResourceTypes{},
		)

	    runScriptCtx, runScriptCancel = context.WithCancel(context.Background())

		runScriptBinPath = "some-bin-path"
		runScriptArgs = []string{ "arg-1", "some-arg2"}
		runScriptInput = []byte(`{
				"source": {"some":"source"},
				"params": {"some":"params"},
				"version": {"some":"version"}
			}`)
		runScriptOutput = make(map[string]string)
		runScriptLogDestination = stderrBuf
		runScriptRecoverable = true

	})

	Context("running", func() {
		BeforeEach(func() {
			fakeGClientContainer.RunStub = func(ctx context.Context, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
				if runErr != nil {
					return nil, runErr
				}

				_, err := io.Stdout.Write([]byte(fakeGardenContainerScriptStdout))
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Stderr.Write([]byte(fakeGardenContainerScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				return scriptProcess, nil
			}

			fakeGClientContainer.AttachStub = func(ctx context.Context, pid string, io garden.ProcessIO) (garden.Process, error) {
				if attachErr != nil {
					return nil, attachErr
				}

				_, err := io.Stdout.Write([]byte(fakeGardenContainerScriptStdout))
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Stderr.Write([]byte(fakeGardenContainerScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				return scriptProcess, nil
			}
		})

		JustBeforeEach(func() {
			runScriptErr = workerContainer.RunScript(
				runScriptCtx,
				runScriptBinPath,
				runScriptArgs,
				runScriptInput,
				&runScriptOutput,
				runScriptLogDestination,
				runScriptRecoverable,
			)
		})

		Context("when a result is already present on the container", func() {
			BeforeEach(func() {
				fakeGClientContainer.PropertiesReturns(garden.Properties{"concourse:resource-result": `{"some-foo-key": "some-foo-value"}`}, nil)
			})

			It("exits successfully", func() {
				Expect(runScriptErr).NotTo(HaveOccurred())
			})

			It("does not run or attach to anything", func() {
				Expect(fakeGClientContainer.RunCallCount()).To(BeZero())
				Expect(fakeGClientContainer.AttachCallCount()).To(BeZero())
			})

			It("can be accessed on the RunScript Output", func(){
				Expect(runScriptOutput).To(HaveKeyWithValue("some-foo-key", "some-foo-value"))
			})
		})

		Context("when the process has already been spawned", func() {
			BeforeEach(func() {
				runScriptInput = []byte(`{
					"bar-key": {"baz":"yarp"}
				}`)
				fakeGClientContainer.PropertiesReturns(nil, nil)
			})

			It("reattaches to it", func() {
				Expect(fakeGClientContainer.AttachCallCount()).To(Equal(1))

				_, pid, io := fakeGClientContainer.AttachArgsForCall(0)
				Expect(pid).To(Equal(runtime.ResourceProcessID))

				// send request on stdin in case process hasn't read it yet
				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(request).To(MatchJSON(`{
					"bar-key": {"baz":"yarp"}
				}`))
			})

			It("does not run an additional process", func() {
				Expect(fakeGClientContainer.RunCallCount()).To(BeZero())
			})

			Context("when the process prints a response", func() {
				BeforeEach(func() {
					fakeGardenContainerScriptStdout = `{"some-key":"with-some-value"}`
				})

				It("can be accessed on the RunScript Output", func(){
					Expect(runScriptOutput).To(HaveKeyWithValue("some-key","with-some-value"))

				})

				It("saves it as a property on the container", func() {
					Expect(fakeGClientContainer.SetPropertyCallCount()).To(Equal(1))

					name, value := fakeGClientContainer.SetPropertyArgsForCall(0)
					Expect(name).To(Equal("concourse:resource-result"))
					Expect(value).To(Equal(fakeGardenContainerScriptStdout))
				})
			})

			Context("when the process outputs to stderr", func() {
				BeforeEach(func() {
					fakeGardenContainerScriptStderr = "some stderr data"
				})

				It("emits it to the log sink", func() {
					Expect(runScriptLogDestination).To(gbytes.Say("some stderr data"))
				})
			})

			Context("when attaching to the process fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					attachErr = disaster
				})

				Context("and run succeeds", func() {
					It("succeeds", func() {
						Expect(runScriptErr).ToNot(HaveOccurred())
					})
				})

				Context("and run subsequently fails", func() {
					BeforeEach(func() {
						runErr = disaster
					})

					It("errors", func() {
						Expect(runScriptErr).To(Equal(disaster))
					})
				})
			})

			Context("when the process exits nonzero", func() {
				BeforeEach(func() {
					scriptExitStatus = 9
				})

				It("returns an err containing stdout/stderr of the process", func() {
					Expect(runScriptErr).To(HaveOccurred())
					Expect(runScriptErr.Error()).To(ContainSubstring("exit status 9"))
				})
			})

		})

		Context("when the process has not yet been spawned", func() {
			BeforeEach(func() {
				fakeGClientContainer.PropertiesReturns(nil, nil)
				attachErr = errors.New("not found")
			})

			It("specifies the process id in the process spec", func() {
				Expect(fakeGClientContainer.RunCallCount()).To(Equal(1))

				_, spec, _ := fakeGClientContainer.RunArgsForCall(0)
				Expect(spec.ID).To(Equal(runtime.ResourceProcessID))
			})

			It("runs the process using <destination (args[0])> with the request as args to stdin", func() {
				Expect(fakeGClientContainer.RunCallCount()).To(Equal(1))

				_, spec, io := fakeGClientContainer.RunArgsForCall(0)
				Expect(spec.Path).To(Equal(runScriptBinPath))
				Expect(spec.Args).To(ConsistOf(runScriptArgs))

				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(request).To(MatchJSON(`{
				"source": {"some":"source"},
				"params": {"some":"params"},
				"version": {"some":"version"}
			}`))
			})

			Context("when process prints the response", func() {
				BeforeEach(func() {
					fakeGardenContainerScriptStdout = `{
					"version": {"some": "new-version"},
					"metadata": [
						{"name": "a", "value":"a-value"},
						{"name": "b","value": "b-value"}
					]
				}`
				})

				// It("can be accessed on the versioned source", func() {
				// 	Expect(versionedSource.Version()).To(Equal(atc.Version{"some": "new-version"}))
				// 	Expect(versionedSource.Metadata()).To(Equal([]atc.MetadataField{
				// 		{Name: "a", Value: "a-value"},
				// 		{Name: "b", Value: "b-value"},
				// 	}))
				// })

				It("saves it as a property on the container", func() {
					Expect(fakeGClientContainer.SetPropertyCallCount()).To(Equal(1))

					name, value := fakeGClientContainer.SetPropertyArgsForCall(0)
					Expect(name).To(Equal("concourse:resource-result"))
					Expect(value).To(Equal(fakeGardenContainerScriptStdout))
				})
			})

			Context("when process outputs to stderr", func() {
				BeforeEach(func() {
					fakeGardenContainerScriptStderr = "some stderr data"
				})

				It("emits it to the log sink", func() {
					Expect(stderrBuf).To(gbytes.Say("some stderr data"))
				})
			})

			Context("when running process fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					runErr = disaster
				})

				It("returns an err", func() {
					Expect(runScriptErr).To(HaveOccurred())
					Expect(runScriptErr).To(Equal(disaster))
				})
			})

			Context("when process exits nonzero", func() {
				BeforeEach(func() {
					scriptExitStatus = 9
				})

				It("returns an err containing stdout/stderr of the process", func() {
					Expect(runScriptErr).To(HaveOccurred())
					Expect(runScriptErr.Error()).To(ContainSubstring("exit status 9"))
				})
			})

			Context("when the process stdout is malformed", func() {
				BeforeEach(func() {
					fakeGardenContainerScriptStdout = "ß"
				})

				It("returns an error", func() {
					Expect(runScriptErr).To(HaveOccurred())
				})

				It("returns original payload in error", func() {
					Expect(runScriptErr.Error()).Should(ContainSubstring(fakeGardenContainerScriptStdout))
				})
			})
		})
	})

	Context("when canceling the context", func() {
		var waited chan<- struct{}
		var done chan struct{}

		BeforeEach(func() {
			fakeGClientContainer.AttachReturns(nil, errors.New("not-found"))
			fakeGClientContainer.RunReturns(scriptProcess, nil)
			fakeGClientContainer.PropertyReturns("", errors.New("nope"))

			waiting := make(chan struct{})
			done = make(chan struct{})
			waited = waiting

			scriptProcess.WaitStub = func() (int, error) {
				// cause waiting to block so that it can be aborted
				<-waiting
				return 0, nil
			}

			fakeGClientContainer.StopStub = func(bool) error {
				close(waited)
				return nil
			}

			go func() {
				runScriptErr = workerContainer.RunScript(
					runScriptCtx,
					runScriptBinPath,
					runScriptArgs,
					runScriptInput,
					&runScriptOutput,
					runScriptLogDestination,
					runScriptRecoverable,
				)

				close(done)
			}()
		})

		It("stops the container", func() {
			runScriptCancel()
			<-done
			Expect(fakeGClientContainer.StopCallCount()).To(Equal(1))
			isStopped := fakeGClientContainer.StopArgsForCall(0)
			Expect(isStopped).To(BeFalse())
		})

		It("doesn't send garden terminate signal to process", func() {
			runScriptCancel()
			<-done
			Expect(runScriptErr).To(Equal(context.Canceled))
			Expect(scriptProcess.SignalCallCount()).To(BeZero())
		})

		Context("when container.stop returns an error", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("gotta get away")

				fakeGClientContainer.StopStub = func(bool) error {
					close(waited)
					return disaster
				}
			})

			It("masks the error", func() {
				runScriptCancel()
				<-done
				Expect(runScriptErr).To(Equal(context.Canceled))
			})
		})
	})
})


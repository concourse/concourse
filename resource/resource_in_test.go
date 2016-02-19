package resource_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/resource"
)

var _ = Describe("Resource In", func() {
	var (
		source  atc.Source
		params  atc.Params
		version atc.Version

		inScriptStdout     string
		inScriptStderr     string
		inScriptExitStatus int
		runInError         error

		inScriptProcess *gfakes.FakeProcess

		versionedSource VersionedSource
		inProcess       ifrit.Process

		ioConfig  IOConfig
		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer
	)

	BeforeEach(func() {
		source = atc.Source{"some": "source"}
		version = atc.Version{"some": "version"}
		params = atc.Params{"some": "params"}

		inScriptStdout = "{}"
		inScriptStderr = ""
		inScriptExitStatus = 0
		runInError = nil

		inScriptProcess = new(gfakes.FakeProcess)
		inScriptProcess.IDReturns("process-id")
		inScriptProcess.WaitStub = func() (int, error) {
			return inScriptExitStatus, nil
		}

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()

		ioConfig = IOConfig{
			Stdout: stdoutBuf,
			Stderr: stderrBuf,
		}

		fakeContainer.PropertiesReturns(garden.Properties{"user": "goose"}, nil)
	})

	itCanStreamOut := func() {
		Describe("streaming bits out", func() {
			Context("when streaming out succeeds", func() {
				BeforeEach(func() {
					fakeContainer.StreamOutStub = func(spec garden.StreamOutSpec) (io.ReadCloser, error) {
						streamOut := new(bytes.Buffer)

						if spec.Path == "/tmp/build/get/some/subdir" {
							streamOut.WriteString("sup")
						}

						return ioutil.NopCloser(streamOut), nil
					}
				})

				It("returns the output stream of the resource directory", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					inStream, err := versionedSource.StreamOut("some/subdir")
					Expect(err).NotTo(HaveOccurred())

					contents, err := ioutil.ReadAll(inStream)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("sup"))
				})
			})

			Context("when streaming out fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					fakeContainer.StreamOutReturns(nil, disaster)
				})

				It("returns the error", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					_, err := versionedSource.StreamOut("some/subdir")
					Expect(err).To(Equal(disaster))
				})
			})
		})
	}

	itStopsOnSignal := func() {
		Context("when a signal is received", func() {
			var waited chan<- struct{}

			BeforeEach(func() {
				waiting := make(chan struct{})
				waited = waiting

				inScriptProcess.WaitStub = func() (int, error) {
					// cause waiting to block so that it can be aborted
					<-waiting
					return 0, nil
				}
			})

			It("stops the container", func() {
				inProcess.Signal(os.Interrupt)

				Eventually(fakeContainer.StopCallCount).Should(Equal(1))

				kill := fakeContainer.StopArgsForCall(0)
				Expect(kill).To(BeFalse())

				close(waited)
			})
		})
	}

	Context("before running /in", func() {
		BeforeEach(func() {
			versionedSource = resource.Get(ioConfig, source, params, version)
		})

		Describe("Version", func() {
			It("returns the version", func() {
				Expect(versionedSource.Version()).To(Equal(atc.Version{"some": "version"}))
			})
		})
	})

	Describe("running", func() {
		JustBeforeEach(func() {
			fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
				if runInError != nil {
					return nil, runInError
				}

				_, err := io.Stdout.Write([]byte(inScriptStdout))
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Stderr.Write([]byte(inScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				return inScriptProcess, nil
			}

			fakeContainer.AttachStub = func(pid string, io garden.ProcessIO) (garden.Process, error) {
				if runInError != nil {
					return nil, runInError
				}

				_, err := io.Stdout.Write([]byte(inScriptStdout))
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Stderr.Write([]byte(inScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				return inScriptProcess, nil
			}

			versionedSource = resource.Get(ioConfig, source, params, version)
			inProcess = ifrit.Invoke(versionedSource)
		})

		AfterEach(func() {
			Eventually(inProcess.Wait()).Should(Receive())
		})

		Context("when a result is already present on the container", func() {
			BeforeEach(func() {
				fakeContainer.PropertyStub = func(name string) (string, error) {
					switch name {
					case "concourse:resource-result":
						return `{
						"version": {"some": "new-version"},
						"metadata": [
							{"name": "a", "value":"a-value"},
							{"name": "b","value": "b-value"}
						]
					}`, nil
					default:
						return "", errors.New("unstubbed property: " + name)
					}
				}
			})

			It("exits successfully", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))
			})

			It("does not run or attach to anything", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				Expect(fakeContainer.RunCallCount()).To(BeZero())
				Expect(fakeContainer.AttachCallCount()).To(BeZero())
			})

			It("can be accessed on the versioned source", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				Expect(versionedSource.Version()).To(Equal(atc.Version{"some": "new-version"}))
				Expect(versionedSource.Metadata()).To(Equal([]atc.MetadataField{
					{Name: "a", Value: "a-value"},
					{Name: "b", Value: "b-value"},
				}))
			})
		})

		Context("when /in has already been spawned", func() {
			BeforeEach(func() {
				fakeContainer.PropertyStub = func(name string) (string, error) {
					switch name {
					case "concourse:resource-process":
						return "process-id", nil
					default:
						return "", errors.New("unstubbed property: " + name)
					}
				}
			})

			It("reattaches to it", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				pid, io := fakeContainer.AttachArgsForCall(0)
				Expect(pid).To(Equal("process-id"))

				// send request on stdin in case process hasn't read it yet
				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(request).To(MatchJSON(`{
					"source": {"some":"source"},
					"params": {"some":"params"},
					"version": {"some":"version"}
				}`))
			})

			It("does not run an additional process", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				Expect(fakeContainer.RunCallCount()).To(BeZero())
			})

			Context("when /opt/resource/in prints the response", func() {
				BeforeEach(func() {
					inScriptStdout = `{
					"version": {"some": "new-version"},
					"metadata": [
						{"name": "a", "value":"a-value"},
						{"name": "b","value": "b-value"}
					]
				}`
				})

				It("can be accessed on the versioned source", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					Expect(versionedSource.Version()).To(Equal(atc.Version{"some": "new-version"}))
					Expect(versionedSource.Metadata()).To(Equal([]atc.MetadataField{
						{Name: "a", Value: "a-value"},
						{Name: "b", Value: "b-value"},
					}))

				})

				It("saves it as a property on the container", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

					name, value := fakeContainer.SetPropertyArgsForCall(0)
					Expect(name).To(Equal("concourse:resource-result"))
					Expect(value).To(Equal(inScriptStdout))
				})
			})

			Context("when /in outputs to stderr", func() {
				BeforeEach(func() {
					inScriptStderr = "some stderr data"
				})

				It("emits it to the log sink", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					Expect(stderrBuf).To(gbytes.Say("some stderr data"))
				})
			})

			Context("when attaching to the process fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					runInError = disaster
				})

				It("returns an err", func() {
					Eventually(inProcess.Wait()).Should(Receive(Equal(disaster)))
				})
			})

			Context("when the process exits nonzero", func() {
				BeforeEach(func() {
					inScriptExitStatus = 9
				})

				It("returns an err containing stdout/stderr of the process", func() {
					var inErr error
					Eventually(inProcess.Wait()).Should(Receive(&inErr))

					Expect(inErr).To(HaveOccurred())
					Expect(inErr.Error()).To(ContainSubstring("exit status 9"))
				})
			})

			itCanStreamOut()
			itStopsOnSignal()
		})

		Context("when /in has not yet been spawned", func() {
			BeforeEach(func() {
				fakeContainer.PropertyStub = func(name string) (string, error) {
					switch name {
					case "concourse:resource-process":
						return "", errors.New("nope")
					default:
						return "", errors.New("unstubbed property: " + name)
					}
				}
			})

			It("uses the same working directory for all actions", func() {
				err := versionedSource.StreamIn("a/path", &bytes.Buffer{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeContainer.StreamInCallCount()).To(Equal(1))
				streamSpec := fakeContainer.StreamInArgsForCall(0)
				Expect(streamSpec.User).To(Equal("")) // use default

				_, err = versionedSource.StreamOut("a/path")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeContainer.StreamOutCallCount()).To(Equal(1))
				streamOutSpec := fakeContainer.StreamOutArgsForCall(0)
				Expect(streamOutSpec.User).To(Equal("")) // use default

				Expect(fakeContainer.RunCallCount()).To(Equal(1))
				spec, _ := fakeContainer.RunArgsForCall(0)

				Expect(streamSpec.Path).To(HavePrefix(spec.Args[0]))
				Expect(streamSpec.Path).To(Equal(streamOutSpec.Path))
			})

			It("runs /opt/resource/in <destination> with the request on stdin", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				spec, io := fakeContainer.RunArgsForCall(0)
				Expect(spec.Path).To(Equal("/opt/resource/in"))

				Expect(spec.Args).To(ConsistOf("/tmp/build/get"))
				Expect(spec.User).To(Equal("goose"))

				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(request).To(MatchJSON(`{
				"source": {"some":"source"},
				"params": {"some":"params"},
				"version": {"some":"version"}
			}`))
			})

			It("saves the process ID as a property", func() {
				Expect(fakeContainer.SetPropertyCallCount()).NotTo(BeZero())

				name, value := fakeContainer.SetPropertyArgsForCall(0)
				Expect(name).To(Equal("concourse:resource-process"))
				Expect(value).To(Equal("process-id"))
			})

			Context("when /opt/resource/in prints the response", func() {
				BeforeEach(func() {
					inScriptStdout = `{
					"version": {"some": "new-version"},
					"metadata": [
						{"name": "a", "value":"a-value"},
						{"name": "b","value": "b-value"}
					]
				}`
				})

				It("can be accessed on the versioned source", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					Expect(versionedSource.Version()).To(Equal(atc.Version{"some": "new-version"}))
					Expect(versionedSource.Metadata()).To(Equal([]atc.MetadataField{
						{Name: "a", Value: "a-value"},
						{Name: "b", Value: "b-value"},
					}))

				})

				It("saves it as a property on the container", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					Expect(fakeContainer.SetPropertyCallCount()).To(Equal(2))

					name, value := fakeContainer.SetPropertyArgsForCall(1)
					Expect(name).To(Equal("concourse:resource-result"))
					Expect(value).To(Equal(inScriptStdout))
				})
			})

			Context("when /in outputs to stderr", func() {
				BeforeEach(func() {
					inScriptStderr = "some stderr data"
				})

				It("emits it to the log sink", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					Expect(stderrBuf).To(gbytes.Say("some stderr data"))
				})
			})

			Context("when running /opt/resource/in fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					runInError = disaster
				})

				It("returns an err", func() {
					Eventually(inProcess.Wait()).Should(Receive(Equal(disaster)))
				})
			})

			Context("when /opt/resource/in exits nonzero", func() {
				BeforeEach(func() {
					inScriptExitStatus = 9
				})

				It("returns an err containing stdout/stderr of the process", func() {
					var inErr error
					Eventually(inProcess.Wait()).Should(Receive(&inErr))

					Expect(inErr).To(HaveOccurred())
					Expect(inErr.Error()).To(ContainSubstring("exit status 9"))
				})
			})

			itCanStreamOut()
			itStopsOnSignal()
		})
	})
})

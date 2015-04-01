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
		inScriptProcess.IDReturns(42)
		inScriptProcess.WaitStub = func() (int, error) {
			return inScriptExitStatus, nil
		}

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()

		ioConfig = IOConfig{
			Stdout: stdoutBuf,
			Stderr: stderrBuf,
		}
	})

	JustBeforeEach(func() {
		fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
			if runInError != nil {
				return nil, runInError
			}

			_, err := io.Stdout.Write([]byte(inScriptStdout))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = io.Stderr.Write([]byte(inScriptStderr))
			Ω(err).ShouldNot(HaveOccurred())

			return inScriptProcess, nil
		}

		fakeContainer.AttachStub = func(pid uint32, io garden.ProcessIO) (garden.Process, error) {
			if runInError != nil {
				return nil, runInError
			}

			_, err := io.Stdout.Write([]byte(inScriptStdout))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = io.Stderr.Write([]byte(inScriptStderr))
			Ω(err).ShouldNot(HaveOccurred())

			return inScriptProcess, nil
		}

		versionedSource = resource.Get(ioConfig, source, params, version)
		inProcess = ifrit.Invoke(versionedSource)
	})

	itCanStreamOut := func() {
		Describe("streaming bits out", func() {
			Context("when streaming out succeeds", func() {
				BeforeEach(func() {
					fakeContainer.StreamOutStub = func(source string) (io.ReadCloser, error) {
						streamOut := new(bytes.Buffer)

						if source == "/tmp/build/src/some/subdir" {
							streamOut.WriteString("sup")
						}

						return ioutil.NopCloser(streamOut), nil
					}
				})

				It("returns the output stream of /tmp/build/src/some-name/", func() {
					Eventually(inProcess.Wait()).Should(Receive(BeNil()))

					inStream, err := versionedSource.StreamOut("some/subdir")
					Ω(err).ShouldNot(HaveOccurred())

					contents, err := ioutil.ReadAll(inStream)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(string(contents)).Should(Equal("sup"))
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
					Ω(err).Should(Equal(disaster))
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
				Ω(kill).Should(BeFalse())

				close(waited)
			})
		})
	}

	Context("when a result is already present on the container", func() {
		BeforeEach(func() {
			fakeContainer.GetPropertyStub = func(name string) (string, error) {
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

			Ω(fakeContainer.RunCallCount()).Should(BeZero())
			Ω(fakeContainer.AttachCallCount()).Should(BeZero())
		})

		It("can be accessed on the versioned source", func() {
			Eventually(inProcess.Wait()).Should(Receive(BeNil()))

			Ω(versionedSource.Version()).Should(Equal(atc.Version{"some": "new-version"}))
			Ω(versionedSource.Metadata()).Should(Equal([]atc.MetadataField{
				{Name: "a", Value: "a-value"},
				{Name: "b", Value: "b-value"},
			}))
		})
	})

	Context("when /in has already been spawned", func() {
		BeforeEach(func() {
			fakeContainer.GetPropertyStub = func(name string) (string, error) {
				switch name {
				case "concourse:resource-process":
					return "42", nil
				default:
					return "", errors.New("unstubbed property: " + name)
				}
			}
		})

		It("reattaches to it", func() {
			Eventually(inProcess.Wait()).Should(Receive(BeNil()))

			pid, io := fakeContainer.AttachArgsForCall(0)
			Ω(pid).Should(Equal(uint32(42)))

			// send request on stdin in case process hasn't read it yet
			request, err := ioutil.ReadAll(io.Stdin)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(request).Should(MatchJSON(`{
				"source": {"some":"source"},
				"params": {"some":"params"},
				"version": {"some":"version"}
			}`))
		})

		It("does not run an additional process", func() {
			Eventually(inProcess.Wait()).Should(Receive(BeNil()))

			Ω(fakeContainer.RunCallCount()).Should(BeZero())
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

				Ω(versionedSource.Version()).Should(Equal(atc.Version{"some": "new-version"}))
				Ω(versionedSource.Metadata()).Should(Equal([]atc.MetadataField{
					{Name: "a", Value: "a-value"},
					{Name: "b", Value: "b-value"},
				}))
			})

			It("saves it as a property on the container", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(1))

				name, value := fakeContainer.SetPropertyArgsForCall(0)
				Ω(name).Should(Equal("concourse:resource-result"))
				Ω(value).Should(Equal(inScriptStdout))
			})
		})

		Context("when /in outputs to stderr", func() {
			BeforeEach(func() {
				inScriptStderr = "some stderr data"
			})

			It("emits it to the log sink", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				Ω(stderrBuf).Should(gbytes.Say("some stderr data"))
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

				Ω(inErr).Should(HaveOccurred())
				Ω(inErr.Error()).Should(ContainSubstring("exit status 9"))
			})
		})

		itCanStreamOut()
		itStopsOnSignal()
	})

	Context("when /in has not yet been spawned", func() {
		BeforeEach(func() {
			fakeContainer.GetPropertyStub = func(name string) (string, error) {
				switch name {
				case "concourse:resource-process":
					return "", errors.New("nope")
				default:
					return "", errors.New("unstubbed property: " + name)
				}
			}
		})

		It("runs /opt/resource/in <destination> with the request on stdin", func() {
			Eventually(inProcess.Wait()).Should(Receive(BeNil()))

			spec, io := fakeContainer.RunArgsForCall(0)
			Ω(spec.Path).Should(Equal("/opt/resource/in"))
			Ω(spec.Args).Should(Equal([]string{"/tmp/build/src"}))
			Ω(spec.Privileged).Should(BeTrue())
			Ω(spec.TTY).Should(Equal(&garden.TTYSpec{}))

			request, err := ioutil.ReadAll(io.Stdin)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(request).Should(MatchJSON(`{
				"source": {"some":"source"},
				"params": {"some":"params"},
				"version": {"some":"version"}
			}`))
		})

		It("saves the process ID as a property", func() {
			Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(1))

			name, value := fakeContainer.SetPropertyArgsForCall(0)
			Ω(name).Should(Equal("concourse:resource-process"))
			Ω(value).Should(Equal("42"))
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

				Ω(versionedSource.Version()).Should(Equal(atc.Version{"some": "new-version"}))
				Ω(versionedSource.Metadata()).Should(Equal([]atc.MetadataField{
					{Name: "a", Value: "a-value"},
					{Name: "b", Value: "b-value"},
				}))
			})

			It("saves it as a property on the container", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(2))

				name, value := fakeContainer.SetPropertyArgsForCall(1)
				Ω(name).Should(Equal("concourse:resource-result"))
				Ω(value).Should(Equal(inScriptStdout))
			})
		})

		Context("when /in outputs to stderr", func() {
			BeforeEach(func() {
				inScriptStderr = "some stderr data"
			})

			It("emits it to the log sink", func() {
				Eventually(inProcess.Wait()).Should(Receive(BeNil()))

				Ω(stderrBuf).Should(gbytes.Say("some stderr data"))
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

				Ω(inErr).Should(HaveOccurred())
				Ω(inErr.Error()).Should(ContainSubstring("exit status 9"))
			})
		})

		itCanStreamOut()
		itStopsOnSignal()
	})
})

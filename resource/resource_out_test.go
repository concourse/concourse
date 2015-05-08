package resource_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/resource/fakes"
)

var _ = Describe("Resource Out", func() {
	var (
		source             atc.Source
		params             atc.Params
		fakeArtifactSource *fakes.FakeArtifactSource

		outScriptStdout     string
		outScriptStderr     string
		outScriptExitStatus int
		runOutError         error

		outScriptProcess *gfakes.FakeProcess

		versionedSource VersionedSource
		outProcess      ifrit.Process

		ioConfig  IOConfig
		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer
	)

	BeforeEach(func() {
		source = atc.Source{"some": "source"}
		params = atc.Params{"some": "params"}
		fakeArtifactSource = new(fakes.FakeArtifactSource)

		outScriptStdout = "{}"
		outScriptStderr = ""
		outScriptExitStatus = 0
		runOutError = nil

		outScriptProcess = new(gfakes.FakeProcess)
		outScriptProcess.IDReturns(42)
		outScriptProcess.WaitStub = func() (int, error) {
			return outScriptExitStatus, nil
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
			if runOutError != nil {
				return nil, runOutError
			}

			_, err := io.Stdout.Write([]byte(outScriptStdout))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = io.Stderr.Write([]byte(outScriptStderr))
			Ω(err).ShouldNot(HaveOccurred())

			return outScriptProcess, nil
		}

		fakeContainer.AttachStub = func(processID uint32, io garden.ProcessIO) (garden.Process, error) {
			if runOutError != nil {
				return nil, runOutError
			}

			_, err := io.Stdout.Write([]byte(outScriptStdout))
			Ω(err).ShouldNot(HaveOccurred())

			_, err = io.Stderr.Write([]byte(outScriptStderr))
			Ω(err).ShouldNot(HaveOccurred())

			return outScriptProcess, nil
		}

		versionedSource = resource.Put(ioConfig, source, params, fakeArtifactSource)
		outProcess = ifrit.Invoke(versionedSource)
	})

	AfterEach(func() {
		Eventually(outProcess.Wait()).Should(Receive())
	})

	itCanStreamOut := func() {
		Describe("streaming bits out", func() {
			Context("when streaming out succeeds", func() {
				BeforeEach(func() {
					fakeContainer.StreamOutStub = func(source string) (io.ReadCloser, error) {
						streamOut := new(bytes.Buffer)

						match, err := regexp.MatchString(`/tmp/build/put/`+guidRegex+`/some/subdir`, source)
						Ω(err).ShouldNot(HaveOccurred())
						if match {
							streamOut.WriteString("sup")
						}

						return ioutil.NopCloser(streamOut), nil
					}
				})

				It("returns the output stream of the resource", func() {
					Eventually(outProcess.Wait()).Should(Receive(BeNil()))

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
					Eventually(outProcess.Wait()).Should(Receive(BeNil()))

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

				outScriptProcess.WaitStub = func() (int, error) {
					// cause waiting to block so that it can be aborted
					<-waiting
					return 0, nil
				}
			})

			It("stops the container", func() {
				outProcess.Signal(os.Interrupt)

				Eventually(fakeContainer.StopCallCount).Should(Equal(1))

				kill := fakeContainer.StopArgsForCall(0)
				Ω(kill).Should(BeFalse())

				close(waited)
			})
		})
	}

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
			Eventually(outProcess.Wait()).Should(Receive(BeNil()))
		})

		It("does not run or attach to anything", func() {
			Eventually(outProcess.Wait()).Should(Receive(BeNil()))

			Ω(fakeContainer.RunCallCount()).Should(BeZero())
			Ω(fakeContainer.AttachCallCount()).Should(BeZero())
		})

		It("can be accessed on the versioned source", func() {
			Eventually(outProcess.Wait()).Should(Receive(BeNil()))

			Ω(versionedSource.Version()).Should(Equal(atc.Version{"some": "new-version"}))
			Ω(versionedSource.Metadata()).Should(Equal([]atc.MetadataField{
				{Name: "a", Value: "a-value"},
				{Name: "b", Value: "b-value"},
			}))
		})
	})

	Context("when /out has already been spawned", func() {
		BeforeEach(func() {
			fakeContainer.PropertyStub = func(name string) (string, error) {
				switch name {
				case "concourse:resource-process":
					return "42", nil
				default:
					return "", errors.New("unstubbed property: " + name)
				}
			}
		})

		It("reattaches to it", func() {
			Eventually(outProcess.Wait()).Should(Receive(BeNil()))

			pid, io := fakeContainer.AttachArgsForCall(0)
			Ω(pid).Should(Equal(uint32(42)))

			// send request on stdin in case process hasn't read it yet
			request, err := ioutil.ReadAll(io.Stdin)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(request).Should(MatchJSON(`{
				"params": {"some":"params"},
				"source": {"some":"source"}
			}`))
		})

		It("does not run an additional process", func() {
			Eventually(outProcess.Wait()).Should(Receive(BeNil()))

			Ω(fakeContainer.RunCallCount()).Should(BeZero())
		})

		It("does not stream the artifact source to the versioned source", func() {
			Ω(fakeArtifactSource.StreamToCallCount()).Should(Equal(0))
		})

		Context("when /opt/resource/out prints the version and metadata", func() {
			BeforeEach(func() {
				outScriptStdout = `{
					"version": {"some": "new-version"},
					"metadata": [
						{"name": "a", "value":"a-value"},
						{"name": "b","value": "b-value"}
					]
				}`
			})

			It("returns the version and metadata printed out by /opt/resource/out", func() {
				Eventually(outProcess.Wait()).Should(Receive(BeNil()))

				Ω(versionedSource.Version()).Should(Equal(atc.Version{"some": "new-version"}))
				Ω(versionedSource.Metadata()).Should(Equal([]atc.MetadataField{
					{Name: "a", Value: "a-value"},
					{Name: "b", Value: "b-value"},
				}))
			})

			It("saves it as a property on the container", func() {
				Eventually(outProcess.Wait()).Should(Receive(BeNil()))

				Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(1))

				name, value := fakeContainer.SetPropertyArgsForCall(0)
				Ω(name).Should(Equal("concourse:resource-result"))
				Ω(value).Should(Equal(outScriptStdout))
			})
		})

		Context("when /out outputs to stderr", func() {
			BeforeEach(func() {
				outScriptStderr = "some stderr data"
			})

			It("emits it to the log sink", func() {
				Eventually(outProcess.Wait()).Should(Receive(BeNil()))

				Ω(stderrBuf).Should(gbytes.Say("some stderr data"))
			})
		})

		Context("when running /opt/resource/out fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				runOutError = disaster
			})

			It("returns the error", func() {
				Eventually(outProcess.Wait()).Should(Receive(Equal(disaster)))
			})
		})

		Context("when /opt/resource/out exits nonzero", func() {
			BeforeEach(func() {
				outScriptExitStatus = 9
			})

			It("returns an err containing stdout/stderr of the process", func() {
				var outErr error
				Eventually(outProcess.Wait()).Should(Receive(&outErr))

				Ω(outErr).Should(HaveOccurred())
				Ω(outErr.Error()).Should(ContainSubstring("exit status 9"))
			})
		})
	})

	Context("when /out has not yet been spawned", func() {
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
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeContainer.StreamInCallCount()).Should(Equal(1))
			streamInPath, _ := fakeContainer.StreamInArgsForCall(0)

			_, err = versionedSource.StreamOut("a/path")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeContainer.StreamOutCallCount()).Should(Equal(1))
			streamOutPath := fakeContainer.StreamOutArgsForCall(0)

			Ω(fakeContainer.RunCallCount()).Should(Equal(1))
			spec, _ := fakeContainer.RunArgsForCall(0)

			Ω(streamInPath).Should(HavePrefix(spec.Args[0]))
			Ω(streamInPath).Should(Equal(streamOutPath))
		})

		It("runs /opt/resource/out <source path> with the request on stdin", func() {
			Eventually(outProcess.Wait()).Should(Receive(BeNil()))

			spec, io := fakeContainer.RunArgsForCall(0)
			Ω(spec.Path).Should(Equal("/opt/resource/out"))
			Ω(spec.Args).Should(ConsistOf(MatchRegexp(`/tmp/build/put/` + guidRegex)))
			Ω(spec.Privileged).Should(BeTrue())

			request, err := ioutil.ReadAll(io.Stdin)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(request).Should(MatchJSON(`{
				"params": {"some":"params"},
				"source": {"some":"source"}
			}`))
		})

		It("streams the artifact source to the versioned source", func() {
			Ω(fakeArtifactSource.StreamToCallCount()).Should(Equal(1))

			dest := fakeArtifactSource.StreamToArgsForCall(0)
			Ω(dest).Should(Equal(versionedSource))
		})

		It("saves the process ID as a property", func() {
			Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(1))

			name, value := fakeContainer.SetPropertyArgsForCall(0)
			Ω(name).Should(Equal("concourse:resource-process"))
			Ω(value).Should(Equal("42"))
		})

		Describe("streaming in", func() {
			Context("when the container can stream in", func() {
				BeforeEach(func() {
					fakeContainer.StreamInReturns(nil)
				})

				It("streams in to the path", func() {
					buf := new(bytes.Buffer)

					err := versionedSource.StreamIn("some-path", buf)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(fakeContainer.StreamInCallCount()).Should(Equal(1))
					dst, src := fakeContainer.StreamInArgsForCall(0)

					Ω(dst).Should(MatchRegexp(`/tmp/build/put/` + guidRegex + `/some-path`))
					Ω(src).Should(Equal(buf))
				})
			})

			Context("when the container cannot stream in", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					fakeContainer.StreamInReturns(disaster)
				})

				It("returns the error", func() {
					err := versionedSource.StreamIn("some-path", nil)
					Ω(err).Should(Equal(disaster))
				})
			})
		})

		Context("when /opt/resource/out prints the version and metadata", func() {
			BeforeEach(func() {
				outScriptStdout = `{
				"version": {"some": "new-version"},
				"metadata": [
					{"name": "a", "value":"a-value"},
					{"name": "b","value": "b-value"}
				]
			}`
			})

			It("returns the version and metadata printed out by /opt/resource/out", func() {
				Eventually(outProcess.Wait()).Should(Receive(BeNil()))

				Ω(versionedSource.Version()).Should(Equal(atc.Version{"some": "new-version"}))
				Ω(versionedSource.Metadata()).Should(Equal([]atc.MetadataField{
					{Name: "a", Value: "a-value"},
					{Name: "b", Value: "b-value"},
				}))
			})

			It("saves it as a property on the container", func() {
				Eventually(outProcess.Wait()).Should(Receive(BeNil()))

				Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(2))

				name, value := fakeContainer.SetPropertyArgsForCall(1)
				Ω(name).Should(Equal("concourse:resource-result"))
				Ω(value).Should(Equal(outScriptStdout))
			})
		})

		Context("when /out outputs to stderr", func() {
			BeforeEach(func() {
				outScriptStderr = "some stderr data"
			})

			It("emits it to the log sink", func() {
				Eventually(outProcess.Wait()).Should(Receive(BeNil()))

				Ω(stderrBuf).Should(gbytes.Say("some stderr data"))
			})
		})

		Context("when running /opt/resource/out fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				runOutError = disaster
			})

			It("returns the error", func() {
				Eventually(outProcess.Wait()).Should(Receive(Equal(disaster)))
			})
		})

		Context("when /opt/resource/out exits nonzero", func() {
			BeforeEach(func() {
				outScriptExitStatus = 9
			})

			It("returns an err containing stdout/stderr of the process", func() {
				var outErr error
				Eventually(outProcess.Wait()).Should(Receive(&outErr))

				Ω(outErr).Should(HaveOccurred())
				Ω(outErr.Error()).Should(ContainSubstring("exit status 9"))
			})
		})

		itCanStreamOut()
		itStopsOnSignal()
	})
})

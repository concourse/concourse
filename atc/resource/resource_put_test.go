package resource_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"

	"code.cloudfoundry.org/garden"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/resource"
)

var _ = Describe("Resource Put", func() {
	var (
		source atc.Source
		params atc.Params

		outScriptStdout     string
		outScriptStderr     string
		outScriptExitStatus int
		runOutError         error
		attachOutError      error
		putErr              error

		outScriptProcess *gfakes.FakeProcess

		versionedSource VersionedSource

		ioConfig  IOConfig
		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		ctx    context.Context
		cancel func()
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		source = atc.Source{"some": "source"}
		params = atc.Params{"some": "params"}

		outScriptStdout = "{}"
		outScriptStderr = ""
		outScriptExitStatus = 0
		runOutError = nil
		attachOutError = nil

		outScriptProcess = new(gfakes.FakeProcess)
		outScriptProcess.IDReturns(TaskProcessID)
		outScriptProcess.WaitStub = func() (int, error) {
			return outScriptExitStatus, nil
		}

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()

		ioConfig = IOConfig{
			Stdout: stdoutBuf,
			Stderr: stderrBuf,
		}
		putErr = nil
	})

	Describe("running", func() {
		JustBeforeEach(func() {
			fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
				if runOutError != nil {
					return nil, runOutError
				}

				_, err := io.Stdout.Write([]byte(outScriptStdout))
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Stderr.Write([]byte(outScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				return outScriptProcess, nil
			}

			fakeContainer.AttachStub = func(processID string, io garden.ProcessIO) (garden.Process, error) {
				if attachOutError != nil {
					return nil, attachOutError
				}

				_, err := io.Stdout.Write([]byte(outScriptStdout))
				Expect(err).NotTo(HaveOccurred())

				_, err = io.Stderr.Write([]byte(outScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				return outScriptProcess, nil
			}

			versionedSource, putErr = resourceForContainer.Put(ctx, ioConfig, source, params)
		})

		itCanStreamOut := func() {
			Describe("streaming bits out", func() {
				Context("when streaming out succeeds", func() {
					BeforeEach(func() {
						fakeContainer.StreamOutStub = func(spec garden.StreamOutSpec) (io.ReadCloser, error) {
							streamOut := new(bytes.Buffer)

							if spec.Path == "/tmp/build/put/some/subdir" {
								streamOut.WriteString("sup")
							}

							return ioutil.NopCloser(streamOut), nil
						}
					})

					It("returns the output stream of the resource", func() {
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
						_, err := versionedSource.StreamOut("some/subdir")
						Expect(err).To(Equal(disaster))
					})
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
				Expect(putErr).NotTo(HaveOccurred())
			})

			It("does not run or attach to anything", func() {
				Expect(fakeContainer.RunCallCount()).To(BeZero())
				Expect(fakeContainer.AttachCallCount()).To(BeZero())
			})

			It("can be accessed on the versioned source", func() {
				Expect(versionedSource.Version()).To(Equal(atc.Version{"some": "new-version"}))
				Expect(versionedSource.Metadata()).To(Equal([]atc.MetadataField{
					{Name: "a", Value: "a-value"},
					{Name: "b", Value: "b-value"},
				}))

			})
		})

		Context("when /out has already been spawned", func() {
			BeforeEach(func() {
				fakeContainer.PropertyStub = func(name string) (string, error) {
					return "", errors.New("unstubbed property: " + name)
				}
			})

			It("reattaches to it", func() {
				Expect(fakeContainer.AttachCallCount()).To(Equal(1))

				pid, io := fakeContainer.AttachArgsForCall(0)
				Expect(pid).To(Equal(TaskProcessID))

				// send request on stdin in case process hasn't read it yet
				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(request).To(MatchJSON(`{
				"params": {"some":"params"},
				"source": {"some":"source"}
			}`))

			})

			It("does not run an additional process", func() {
				Expect(fakeContainer.RunCallCount()).To(BeZero())
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
					Expect(versionedSource.Version()).To(Equal(atc.Version{"some": "new-version"}))
					Expect(versionedSource.Metadata()).To(Equal([]atc.MetadataField{
						{Name: "a", Value: "a-value"},
						{Name: "b", Value: "b-value"},
					}))

				})

				It("saves it as a property on the container", func() {
					Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

					name, value := fakeContainer.SetPropertyArgsForCall(0)
					Expect(name).To(Equal("concourse:resource-result"))
					Expect(value).To(Equal(outScriptStdout))
				})
			})

			Context("when /out outputs to stderr", func() {
				BeforeEach(func() {
					outScriptStderr = "some stderr data"
				})

				It("emits it to the log sink", func() {
					Expect(stderrBuf).To(gbytes.Say("some stderr data"))
				})
			})

			Context("when running /opt/resource/out fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					attachOutError = disaster
					runOutError = disaster
				})

				It("returns the error", func() {
					Expect(putErr).To(HaveOccurred())
					Expect(putErr).To(Equal(disaster))
				})
			})

			Context("when /opt/resource/out exits nonzero", func() {
				BeforeEach(func() {
					outScriptExitStatus = 9
				})

				It("returns an err containing stdout/stderr of the process", func() {
					Expect(putErr).To(HaveOccurred())
					Expect(putErr.Error()).To(ContainSubstring("exit status 9"))
				})
			})
		})

		Context("when /out has not yet been spawned", func() {
			BeforeEach(func() {
				fakeContainer.PropertyStub = func(name string) (string, error) {
					switch name {
					default:
						return "", errors.New("unstubbed property: " + name)
					}
				}

				attachOutError = errors.New("not-found")
			})

			It("specifies the process id in the process spec", func() {
				Expect(fakeContainer.RunCallCount()).To(Equal(1))

				spec, _ := fakeContainer.RunArgsForCall(0)
				Expect(spec.ID).To(Equal(TaskProcessID))
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

				Expect(fakeContainer.RunCallCount()).To(Equal(1))
				spec, _ := fakeContainer.RunArgsForCall(0)

				Expect(streamSpec.Path).To(HavePrefix(spec.Args[0]))
				Expect(streamSpec.Path).To(Equal(streamOutSpec.Path))
			})

			It("runs /opt/resource/out <source path> with the request on stdin", func() {
				Expect(fakeContainer.RunCallCount()).To(Equal(1))

				spec, io := fakeContainer.RunArgsForCall(0)
				Expect(spec.Path).To(Equal("/opt/resource/out"))
				Expect(spec.Args).To(ConsistOf("/tmp/build/put"))

				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(request).To(MatchJSON(`{
				"params": {"some":"params"},
				"source": {"some":"source"}
			}`))

			})

			Describe("streaming in", func() {
				Context("when the container can stream in", func() {
					BeforeEach(func() {
						fakeContainer.StreamInReturns(nil)
					})

					It("streams in to the path", func() {
						buf := new(bytes.Buffer)

						err := versionedSource.StreamIn("some-path", buf)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakeContainer.StreamInCallCount()).To(Equal(1))
						spec := fakeContainer.StreamInArgsForCall(0)

						Expect(spec.Path).To(Equal("/tmp/build/put/some-path"))
						Expect(spec.User).To(Equal("")) // use default
						Expect(spec.TarStream).To(Equal(buf))
					})
				})

				Context("when the container cannot stream in", func() {
					disaster := errors.New("oh no!")

					BeforeEach(func() {
						fakeContainer.StreamInReturns(disaster)
					})

					It("returns the error", func() {
						err := versionedSource.StreamIn("some-path", nil)
						Expect(err).To(Equal(disaster))
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
					Expect(versionedSource.Version()).To(Equal(atc.Version{"some": "new-version"}))
					Expect(versionedSource.Metadata()).To(Equal([]atc.MetadataField{
						{Name: "a", Value: "a-value"},
						{Name: "b", Value: "b-value"},
					}))

				})

				It("saves it as a property on the container", func() {
					Expect(fakeContainer.SetPropertyCallCount()).To(Equal(1))

					name, value := fakeContainer.SetPropertyArgsForCall(0)
					Expect(name).To(Equal("concourse:resource-result"))
					Expect(value).To(Equal(outScriptStdout))
				})
			})

			Context("when /out outputs to stderr", func() {
				BeforeEach(func() {
					outScriptStderr = "some stderr data"
				})

				It("emits it to the log sink", func() {
					Expect(stderrBuf).To(gbytes.Say("some stderr data"))
				})
			})

			Context("when running /opt/resource/out fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					runOutError = disaster
				})

				It("returns the error", func() {
					Expect(putErr).To(HaveOccurred())
					Expect(putErr).To(Equal(disaster))
				})
			})

			Context("when /opt/resource/out exits nonzero", func() {
				BeforeEach(func() {
					outScriptExitStatus = 9
				})

				It("returns an err containing stdout/stderr of the process", func() {
					Expect(putErr).To(HaveOccurred())
					Expect(putErr.Error()).To(ContainSubstring("exit status 9"))
				})
			})

			itCanStreamOut()
		})
	})

	Context("when a signal is received", func() {
		var waited chan<- struct{}
		var done chan struct{}

		BeforeEach(func() {
			fakeContainer.AttachReturns(nil, errors.New("not-found"))
			fakeContainer.RunReturns(outScriptProcess, nil)
			fakeContainer.PropertyReturns("", errors.New("nope"))

			waiting := make(chan struct{})
			done = make(chan struct{})
			waited = waiting

			outScriptProcess.WaitStub = func() (int, error) {
				// cause waiting to block so that it can be aborted
				<-waiting
				return 0, nil
			}

			fakeContainer.StopStub = func(bool) error {
				close(waited)
				return nil
			}

			go func() {
				versionedSource, putErr = resourceForContainer.Put(ctx, ioConfig, source, params)
				close(done)
			}()
		})

		It("stops the container", func() {
			cancel()
			<-done
			Expect(fakeContainer.StopCallCount()).To(Equal(1))
			Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
			Expect(putErr).To(Equal(context.Canceled))
		})

		It("doesn't send garden terminate signal to process", func() {
			cancel()
			<-done
			Expect(putErr).To(Equal(context.Canceled))
			Expect(outScriptProcess.SignalCallCount()).To(BeZero())
		})

		Context("when container.stop returns an error", func() {
			var disaster error

			BeforeEach(func() {
				disaster = errors.New("gotta get away")

				fakeContainer.StopStub = func(bool) error {
					close(waited)
					return disaster
				}
			})

			It("masks the error", func() {
				cancel()
				<-done
				Expect(putErr).To(Equal(context.Canceled))
			})
		})
	})
})

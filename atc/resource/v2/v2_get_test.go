package v2_test

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/concourse/atc"
	v2 "github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/resource/v2/v2fakes"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource Get", func() {
	var (
		config map[string]interface{}

		source  atc.Source
		params  atc.Params
		version atc.Version
		space   atc.Space

		inScriptStderr     string
		inScriptExitStatus int
		runInError         error
		attachInError      error

		inScriptProcess *gardenfakes.FakeProcess

		fakeGetEventHandler *v2fakes.FakeGetEventHandler

		ioConfig  atc.IOConfig
		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		fakeVolume *workerfakes.FakeVolume
		response   []byte

		ctx    context.Context
		cancel func()

		getErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeGetEventHandler = new(v2fakes.FakeGetEventHandler)

		source = atc.Source{"some": "source"}
		version = atc.Version{"some": "version"}
		params = atc.Params{"other": "params"}
		space = atc.Space("some-space")

		config = make(map[string]interface{})
		for k, v := range source {
			config[k] = v
		}
		for k, v := range params {
			config[k] = v
		}

		inScriptStderr = ""
		inScriptExitStatus = 0
		runInError = nil
		attachInError = nil
		getErr = nil

		inScriptProcess = new(gardenfakes.FakeProcess)
		inScriptProcess.IDReturns(v2.ResourceProcessID)
		inScriptProcess.WaitStub = func() (int, error) {
			return inScriptExitStatus, nil
		}

		stdoutBuf = gbytes.NewBuffer()
		stderrBuf = gbytes.NewBuffer()

		ioConfig = atc.IOConfig{
			Stdout: stdoutBuf,
			Stderr: stderrBuf,
		}

		fakeVolume = new(workerfakes.FakeVolume)

		streamedOut := gbytes.NewBuffer()
		fakeContainer.StreamOutReturns(streamedOut, nil)

		response = []byte(`
			{"action": "fetched", "space": "some-space", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}`)
	})

	Describe("running", func() {
		JustBeforeEach(func() {
			fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
				if runInError != nil {
					return nil, runInError
				}

				_, err := io.Stderr.Write([]byte(inScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				var getReq v2.GetRequest
				err = json.Unmarshal(request, &getReq)
				Expect(err).NotTo(HaveOccurred())

				Expect(getReq.Config).To(Equal(map[string]interface{}(config)))
				Expect(getReq.Space).To(Equal(space))
				Expect(getReq.Version).To(Equal(version))
				Expect(getReq.ResponsePath).ToNot(BeEmpty())

				return inScriptProcess, nil
			}

			fakeContainer.AttachStub = func(pid string, io garden.ProcessIO) (garden.Process, error) {
				if attachInError != nil {
					return nil, attachInError
				}

				_, err := io.Stderr.Write([]byte(inScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				var getReq v2.GetRequest
				err = json.Unmarshal(request, &getReq)
				Expect(err).NotTo(HaveOccurred())

				Expect(getReq.Config).To(Equal(map[string]interface{}(config)))
				Expect(getReq.Space).To(Equal(space))
				Expect(getReq.Version).To(Equal(version))
				Expect(getReq.ResponsePath).ToNot(BeEmpty())

				return inScriptProcess, nil
			}

			getErr = resource.Get(ctx, fakeGetEventHandler, fakeVolume, ioConfig, source, params, space, version)
		})

		Context("when artifact get has already been spawned", func() {
			It("reattaches to it", func() {
				Expect(fakeContainer.AttachCallCount()).To(Equal(1))

				pid, _ := fakeContainer.AttachArgsForCall(0)
				Expect(pid).To(Equal(v2.ResourceProcessID))
			})

			It("does not run an additional process", func() {
				Expect(fakeContainer.RunCallCount()).To(BeZero())
			})

			Context("when artifact get succeeds", func() {
				BeforeEach(func() {
					tarStream := new(bytes.Buffer)

					tarWriter := tar.NewWriter(tarStream)

					err := tarWriter.WriteHeader(&tar.Header{
						Name: "doesnt matter",
						Size: int64(len(response)),
					})
					Expect(err).ToNot(HaveOccurred())

					_, err = tarWriter.Write(response)
					Expect(err).ToNot(HaveOccurred())

					err = tarWriter.Close()
					Expect(err).ToNot(HaveOccurred())

					fakeContainer.StreamOutReturns(ioutil.NopCloser(tarStream), nil)
				})

				It("returns the versions and space written to the temp file", func() {
					Expect(fakeGetEventHandler.SaveMetadataCallCount()).To(Equal(1))
					metadata := fakeGetEventHandler.SaveMetadataArgsForCall(0)
					Expect(metadata).To(Equal(atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					}))

					Expect(getErr).ToNot(HaveOccurred())
				})
			})

			Context("when artifact get outputs to stderr", func() {
				BeforeEach(func() {
					inScriptStderr = "some stderr data"
				})

				It("emits it to the log sink", func() {
					Expect(stderrBuf).To(gbytes.Say("some stderr data"))
				})
			})

			Context("when attaching to the process fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					attachInError = disaster
					runInError = disaster
				})

				It("errors", func() {
					Expect(getErr).To(HaveOccurred())
					Expect(getErr).To(Equal(disaster))
				})
			})

			Context("when the process exits nonzero", func() {
				BeforeEach(func() {
					inScriptExitStatus = 9
				})

				It("returns an err containing stdout/stderr of the process", func() {
					Expect(getErr).To(HaveOccurred())
					Expect(getErr.Error()).To(ContainSubstring("exit status 9"))
				})
			})
		})

		Context("when artifact get has not yet been spawned", func() {
			BeforeEach(func() {
				attachInError = errors.New("not found")
			})

			It("specifies the process id in the process spec", func() {
				Expect(fakeContainer.RunCallCount()).To(Equal(1))

				spec, _ := fakeContainer.RunArgsForCall(0)
				Expect(spec.ID).To(Equal(v2.ResourceProcessID))
			})

			Context("when artifact get succeeds", func() {
				BeforeEach(func() {
					tarStream := new(bytes.Buffer)

					tarWriter := tar.NewWriter(tarStream)

					err := tarWriter.WriteHeader(&tar.Header{
						Name: "doesnt matter",
						Size: int64(len(response)),
					})
					Expect(err).ToNot(HaveOccurred())

					_, err = tarWriter.Write(response)
					Expect(err).ToNot(HaveOccurred())

					err = tarWriter.Close()
					Expect(err).ToNot(HaveOccurred())

					fakeContainer.StreamOutReturns(ioutil.NopCloser(tarStream), nil)
				})

				It("returns the version, metadata and space written to the temp file", func() {
					Expect(getErr).ToNot(HaveOccurred())
				})
			})

			It("runs artifact get in <destination> with the request on stdin", func() {
				Expect(fakeContainer.RunCallCount()).To(Equal(1))

				spec, _ := fakeContainer.RunArgsForCall(0)
				Expect(spec.Path).To(Equal(resourceInfo.Artifacts.Get))
				Expect(spec.Dir).To(Equal("get"))
			})

			Context("when artifact get outputs to stderr", func() {
				BeforeEach(func() {
					inScriptStderr = "some stderr data"
				})

				It("emits it to the log sink", func() {
					Expect(stderrBuf).To(gbytes.Say("some stderr data"))
				})
			})

			Context("when running artifact get fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					runInError = disaster
				})

				It("returns an err", func() {
					Expect(getErr).To(HaveOccurred())
					Expect(getErr).To(Equal(disaster))
				})
			})

			Context("when artifact get exits nonzero", func() {
				BeforeEach(func() {
					inScriptExitStatus = 9
				})

				It("returns an err containing stdout/stderr of the process", func() {
					Expect(getErr).To(HaveOccurred())
					Expect(getErr.Error()).To(ContainSubstring("exit status 9"))
				})
			})

			Context("when the response writes multiple versions to the temp file", func() {
				BeforeEach(func() {
					tarStream := new(bytes.Buffer)

					tarWriter := tar.NewWriter(tarStream)

					response = []byte(`
			{"action": "fetched", "space": "some-space", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}
			{"action": "fetched", "space": "second-space", "version": {"ref": "v2"}, "metadata": [{"name": "second", "value": "metadata"}]}`)

					err := tarWriter.WriteHeader(&tar.Header{
						Name: "doesnt matter",
						Size: int64(len(response)),
					})
					Expect(err).ToNot(HaveOccurred())

					_, err = tarWriter.Write(response)
					Expect(err).ToNot(HaveOccurred())

					err = tarWriter.Close()
					Expect(err).ToNot(HaveOccurred())

					fakeContainer.StreamOutReturns(ioutil.NopCloser(tarStream), nil)
				})

				It("only saves the metadata once for the first version and ignores the rest", func() {
					Expect(fakeGetEventHandler.SaveMetadataCallCount()).To(Equal(1))
					metadata := fakeGetEventHandler.SaveMetadataArgsForCall(0)
					Expect(metadata).To(Equal(atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					}))

					Expect(getErr).ToNot(HaveOccurred())
				})
			})

			Context("when the response is garbage", func() {
				BeforeEach(func() {
					tarStream := new(bytes.Buffer)

					tarWriter := tar.NewWriter(tarStream)

					response = []byte("vito")

					err := tarWriter.WriteHeader(&tar.Header{
						Name: "doesnt matter",
						Size: int64(len(response)),
					})
					Expect(err).ToNot(HaveOccurred())

					_, err = tarWriter.Write(response)
					Expect(err).ToNot(HaveOccurred())

					err = tarWriter.Close()
					Expect(err).ToNot(HaveOccurred())

					fakeContainer.StreamOutReturns(ioutil.NopCloser(tarStream), nil)
				})

				It("returns a failed to decode error", func() {
					Expect(getErr).To(HaveOccurred())
					Expect(getErr.Error()).To(ContainSubstring("failed to decode response"))
				})
			})

			Context("when streaming out fails", func() {
				BeforeEach(func() {
					fakeContainer.StreamOutReturns(nil, errors.New("ah"))
				})

				It("returns the error", func() {
					Expect(getErr).To(HaveOccurred())
				})
			})

			Context("when streaming out non tar response", func() {
				BeforeEach(func() {
					streamedOut := gbytes.NewBuffer()
					fakeContainer.StreamOutReturns(streamedOut, nil)
				})

				It("returns an error", func() {
					Expect(getErr).To(HaveOccurred())
				})
			})
		})
	})

	Context("when canceling the context", func() {
		var waited chan<- struct{}
		var done chan struct{}

		BeforeEach(func() {
			fakeContainer.AttachReturns(nil, errors.New("not-found"))
			fakeContainer.RunReturns(inScriptProcess, nil)

			waiting := make(chan struct{})
			done = make(chan struct{})
			waited = waiting

			inScriptProcess.WaitStub = func() (int, error) {
				// cause waiting to block so that it can be aborted
				<-waiting
				return 0, nil
			}

			fakeContainer.StopStub = func(bool) error {
				close(waited)
				return nil
			}

			go func() {
				getErr = resource.Get(ctx, fakeGetEventHandler, fakeVolume, ioConfig, source, params, space, version)
				close(done)
			}()
		})

		It("stops the container", func() {
			cancel()
			<-done
			Expect(fakeContainer.StopCallCount()).To(Equal(1))
			Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
		})

		It("doesn't send garden terminate signal to process", func() {
			cancel()
			<-done
			Expect(getErr).To(Equal(context.Canceled))
			Expect(inScriptProcess.SignalCallCount()).To(BeZero())
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
				Expect(getErr).To(Equal(context.Canceled))
			})
		})
	})
})

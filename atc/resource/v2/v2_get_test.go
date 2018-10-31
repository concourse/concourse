package v2_test

import (
	"context"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Resource Get", func() {
	var (
		source  atc.Source
		params  atc.Params
		version atc.Version
		space   atc.Space

		inScriptStderr     string
		inScriptExitStatus int
		runInError         error
		attachInError      error

		inScriptProcess *gardenfakes.FakeProcess

		ioConfig  atc.IOConfig
		stdoutBuf *gbytes.Buffer
		stderrBuf *gbytes.Buffer

		fakeVolume *workerfakes.FakeVolume

		ctx    context.Context
		cancel func()

		getErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		source = atc.Source{"some": "source"}
		version = atc.Version{"some": "version"}
		params = atc.Params{"other": "params"}
		space = atc.Space("some-space")

		inScriptStderr = ""
		inScriptExitStatus = 0
		runInError = nil
		attachInError = nil
		getErr = nil

		inScriptProcess = new(gardenfakes.FakeProcess)
		inScriptProcess.IDReturns(v2.TaskProcessID)
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
	})

	Describe("running", func() {
		JustBeforeEach(func() {
			fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
				if runInError != nil {
					return nil, runInError
				}

				_, err := io.Stderr.Write([]byte(inScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				return inScriptProcess, nil
			}

			fakeContainer.AttachStub = func(pid string, io garden.ProcessIO) (garden.Process, error) {
				if attachInError != nil {
					return nil, attachInError
				}

				_, err := io.Stderr.Write([]byte(inScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				return inScriptProcess, nil
			}

			getErr = resource.Get(ctx, fakeVolume, ioConfig, source, params, space, version)
		})

		Context("when artifact get has already been spawned", func() {
			It("reattaches to it", func() {
				Expect(fakeContainer.AttachCallCount()).To(Equal(1))

				pid, io := fakeContainer.AttachArgsForCall(0)
				Expect(pid).To(Equal(v2.TaskProcessID))

				// send request on stdin in case process hasn't read it yet
				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(request).To(MatchJSON(`{
					"config": {"some":"source","other":"params"},
					"version": {"some":"version"},
					"space": "some-space"
				}`))
			})

			It("does not run an additional process", func() {
				Expect(fakeContainer.RunCallCount()).To(BeZero())
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
				})

				Context("and run succeeds", func() {
					It("succeeds", func() {
						Expect(getErr).ToNot(HaveOccurred())
					})
				})

				Context("and run subsequently fails", func() {
					BeforeEach(func() {
						runInError = disaster
					})

					It("errors", func() {
						Expect(getErr).To(HaveOccurred())
						Expect(getErr).To(Equal(disaster))
					})
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
				Expect(spec.ID).To(Equal(v2.TaskProcessID))
			})

			It("runs artifact get in <destination> with the request on stdin", func() {
				Expect(fakeContainer.RunCallCount()).To(Equal(1))

				spec, io := fakeContainer.RunArgsForCall(0)
				Expect(spec.Path).To(Equal(resourceInfo.Artifacts.Get))
				Expect(spec.Args).To(ConsistOf("/tmp/build/get"))

				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				Expect(request).To(MatchJSON(`{
					"config": {"some":"source","other":"params"},
					"version": {"some":"version"},
					"space": "some-space"
				}`))
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
				getErr = resource.Get(ctx, fakeVolume, ioConfig, source, params, space, version)
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

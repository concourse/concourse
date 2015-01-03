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

	garden "github.com/cloudfoundry-incubator/garden/api"
	gfakes "github.com/cloudfoundry-incubator/garden/api/fakes"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/exec/resource"
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

		versionedSource = resource.Get(ioConfig, source, params, version)
		inProcess = ifrit.Invoke(versionedSource)
	})

	It("runs /opt/resource/in <destination> with the request on stdin", func() {
		Eventually(inProcess.Wait()).Should(Receive(BeNil()))

		spec, io := fakeContainer.RunArgsForCall(0)
		Ω(spec.Path).Should(Equal("/opt/resource/in"))
		Ω(spec.Args).Should(Equal([]string{"/tmp/build/src"}))
		Ω(spec.Privileged).Should(BeTrue())

		request, err := ioutil.ReadAll(io.Stdin)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(request).Should(MatchJSON(`{
			"source": {"some":"source"},
			"params": {"some":"params"},
			"version": {"some":"version"}
		}`))
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

	// Context("when a config path is specified", func() {
	// 	BeforeEach(func() {
	// 		input.ConfigPath = "some/config/path.yml"
	// 	})
	//
	// 	Context("and the config path exists", func() {
	// 		BeforeEach(func() {
	// 			fakeContainer.StreamOutStub = func(src string) (io.ReadCloser, error) {
	// 				buf := new(bytes.Buffer)
	//
	// 				if src == "/tmp/build/src/some-name/some/config/path.yml" {
	// 					tarWriter := tar.NewWriter(buf)
	//
	// 					contents := []byte("---\nimage: some-reconfigured-image\n")
	//
	// 					tarWriter.WriteHeader(&tar.Header{
	// 						Name: "./doesnt-matter",
	// 						Mode: 0644,
	// 						Size: int64(len(contents)),
	// 					})
	//
	// 					tarWriter.Write(contents)
	// 				}
	//
	// 				return ioutil.NopCloser(buf), nil
	// 			}
	// 		})
	//
	// 		It("is parsed and returned as a Build", func() {
	// 			Ω(inConfig.Image).Should(Equal("some-reconfigured-image"))
	// 		})
	//
	// 		Context("but the output is invalid", func() {
	// 			BeforeEach(func() {
	// 				fakeContainer.StreamOutStub = func(src string) (io.ReadCloser, error) {
	// 					buf := new(bytes.Buffer)
	//
	// 					if src == "/tmp/build/src/some-name/some/config/path.yml" {
	// 						tarWriter := tar.NewWriter(buf)
	//
	// 						contents := []byte("[")
	//
	// 						tarWriter.WriteHeader(&tar.Header{
	// 							Name: "./doesnt-matter",
	// 							Mode: 0644,
	// 							Size: int64(len(contents)),
	// 						})
	//
	// 						tarWriter.Write(contents)
	// 					}
	//
	// 					return ioutil.NopCloser(buf), nil
	// 				}
	// 			})
	//
	// 			It("returns an error", func() {
	// 				Ω(inErr).Should(HaveOccurred())
	// 			})
	// 		})
	// 	})
	//
	// 	Context("when the config cannot be fetched", func() {
	// 		disaster := errors.New("oh no!")
	//
	// 		BeforeEach(func() {
	// 			fakeContainer.StreamOutReturns(nil, disaster)
	// 		})
	//
	// 		It("returns the error", func() {
	// 			Ω(inErr).Should(Equal(disaster))
	// 		})
	// 	})
	//
	// 	Context("when the config path does not exist", func() {
	// 		BeforeEach(func() {
	// 			fakeContainer.StreamOutStub = func(string, string) (io.ReadCloser, error) {
	// 				return ioutil.NopCloser(new(bytes.Buffer)), nil
	// 			}
	// 		})
	//
	// 		It("returns an error", func() {
	// 			Ω(inErr).Should(HaveOccurred())
	// 		})
	// 	})
	// })

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
			inScriptStdout = "some-stdout-data"
			inScriptStderr = "some-stderr-data"
			inScriptExitStatus = 9
		})

		It("returns an err containing stdout/stderr of the process", func() {
			var inErr error
			Eventually(inProcess.Wait()).Should(Receive(&inErr))

			Ω(inErr).Should(HaveOccurred())
			Ω(inErr.Error()).Should(ContainSubstring("some-stdout-data"))
			Ω(inErr.Error()).Should(ContainSubstring("some-stderr-data"))
			Ω(inErr.Error()).Should(ContainSubstring("exit status 9"))
		})
	})

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
})

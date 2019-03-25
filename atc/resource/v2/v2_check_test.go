package v2_test

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/concourse/atc"
	v2 "github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/resource/v2/v2fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Resource Check", func() {
	var (
		source       atc.Source
		spaceVersion map[atc.Space]atc.Version

		checkScriptStderr     string
		checkScriptExitStatus int
		runCheckError         error
		attachCheckError      error

		checkScriptProcess    *gardenfakes.FakeProcess
		fakeCheckEventHandler *v2fakes.FakeCheckEventHandler

		checkErr error
		response []byte

		ctx    context.Context
		cancel func()
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		source = atc.Source{"some": "source"}
		fakeCheckEventHandler = new(v2fakes.FakeCheckEventHandler)
		spaceVersion = map[atc.Space]atc.Version{"space": atc.Version{"some": "version"}}

		checkScriptStderr = ""
		checkScriptExitStatus = 0
		runCheckError = nil
		attachCheckError = nil
		checkErr = nil

		checkScriptProcess = new(gardenfakes.FakeProcess)
		checkScriptProcess.IDReturns(v2.ResourceProcessID)
		checkScriptProcess.WaitStub = func() (int, error) {
			return checkScriptExitStatus, nil
		}

		streamedOut := gbytes.NewBuffer()
		fakeContainer.StreamOutReturns(streamedOut, nil)

		response = []byte(`
		{"action": "default_space", "space": "space"}
		{"action": "discovered", "space": "space", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}
		{"action": "discovered", "space": "space", "version": {"ref": "v2"}, "metadata": [{"name": "some", "value": "metadata"}]}
		{"action": "discovered", "space": "space2", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}`)
	})

	Describe("running", func() {
		JustBeforeEach(func() {
			fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
				if runCheckError != nil {
					return nil, runCheckError
				}

				_, err := io.Stderr.Write([]byte(checkScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				var checkReq v2.CheckRequest
				err = json.Unmarshal(request, &checkReq)
				Expect(err).NotTo(HaveOccurred())

				Expect(checkReq.Config).To(Equal(map[string]interface{}(source)))
				Expect(checkReq.From).To(Equal(spaceVersion))
				Expect(checkReq.ResponsePath).ToNot(BeEmpty())

				return checkScriptProcess, nil
			}

			fakeContainer.AttachStub = func(pid string, io garden.ProcessIO) (garden.Process, error) {
				if attachCheckError != nil {
					return nil, attachCheckError
				}

				_, err := io.Stderr.Write([]byte(checkScriptStderr))
				Expect(err).NotTo(HaveOccurred())

				request, err := ioutil.ReadAll(io.Stdin)
				Expect(err).NotTo(HaveOccurred())

				var checkReq v2.CheckRequest
				err = json.Unmarshal(request, &checkReq)
				Expect(err).NotTo(HaveOccurred())

				Expect(checkReq.Config).To(Equal(map[string]interface{}(source)))
				Expect(checkReq.From).To(Equal(spaceVersion))
				Expect(checkReq.ResponsePath).ToNot(BeEmpty())

				return checkScriptProcess, nil
			}

			checkErr = resource.Check(ctx, fakeCheckEventHandler, source, spaceVersion)
		})

		Context("when check artifact has already been spawned", func() {
			It("reattaches to it", func() {
				Expect(fakeContainer.AttachCallCount()).To(Equal(1))

				pid, _ := fakeContainer.AttachArgsForCall(0)
				Expect(pid).To(Equal(v2.ResourceProcessID))
			})

			It("does not run an additional process", func() {
				Expect(fakeContainer.RunCallCount()).To(BeZero())
			})

			Context("when artifact check succeeds", func() {
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

				It("saves the default space, versions, all spaces and latest versions for each space", func() {
					Expect(fakeCheckEventHandler.DefaultSpaceCallCount()).To(Equal(1))
					Expect(fakeCheckEventHandler.DefaultSpaceArgsForCall(0)).To(Equal(atc.Space("space")))

					Expect(fakeCheckEventHandler.DiscoveredCallCount()).To(Equal(3))
					space, version, metadata := fakeCheckEventHandler.DiscoveredArgsForCall(0)
					Expect(space).To(Equal(atc.Space("space")))
					Expect(version).To(Equal(atc.Version{"ref": "v1"}))
					Expect(metadata).To(Equal(atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					}))

					space, version, metadata = fakeCheckEventHandler.DiscoveredArgsForCall(1)
					Expect(space).To(Equal(atc.Space("space")))
					Expect(version).To(Equal(atc.Version{"ref": "v2"}))
					Expect(metadata).To(Equal(atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					}))

					space, version, metadata = fakeCheckEventHandler.DiscoveredArgsForCall(2)
					Expect(space).To(Equal(atc.Space("space2")))
					Expect(version).To(Equal(atc.Version{"ref": "v1"}))
					Expect(metadata).To(Equal(atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					}))

					Expect(fakeCheckEventHandler.LatestVersionsCallCount()).To(Equal(1))

					Expect(checkErr).ToNot(HaveOccurred())
				})
			})

			Context("when running artifact check fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					attachCheckError = disaster
					runCheckError = disaster
				})

				It("returns the error", func() {
					Expect(checkErr).To(HaveOccurred())
					Expect(checkErr).To(Equal(disaster))
				})
			})

			Context("when artifact check exits nonzero", func() {
				BeforeEach(func() {
					checkScriptStderr = "some-stderr"
					checkScriptExitStatus = 9
				})

				It("returns an error containing stderr of the process", func() {
					Expect(checkErr).To(HaveOccurred())

					Expect(checkErr.Error()).To(ContainSubstring("exit status 9"))
					Expect(checkErr.Error()).To(ContainSubstring("some-stderr"))
				})
			})
		})

		Context("when artifact check has not yet been spawned", func() {
			BeforeEach(func() {
				attachCheckError = errors.New("not-found")
			})

			It("specifies the process id in the process spec", func() {
				Expect(fakeContainer.RunCallCount()).To(Equal(1))

				spec, _ := fakeContainer.RunArgsForCall(0)
				Expect(spec.ID).To(Equal(v2.ResourceProcessID))
			})

			It("runs check artifact with the request on stdin", func() {
				Expect(fakeContainer.RunCallCount()).To(Equal(1))

				spec, _ := fakeContainer.RunArgsForCall(0)
				Expect(spec.Path).To(Equal(resourceInfo.Artifacts.Check))
				Expect(spec.Dir).To(Equal("check"))
			})

			Context("when artifact check succeeds", func() {
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

				It("saves the default space, versions, all spaces and latest versions for each space", func() {
					Expect(fakeCheckEventHandler.DefaultSpaceCallCount()).To(Equal(1))
					Expect(fakeCheckEventHandler.DefaultSpaceArgsForCall(0)).To(Equal(atc.Space("space")))

					Expect(fakeCheckEventHandler.DiscoveredCallCount()).To(Equal(3))
					space, version, metadata := fakeCheckEventHandler.DiscoveredArgsForCall(0)
					Expect(space).To(Equal(atc.Space("space")))
					Expect(version).To(Equal(atc.Version{"ref": "v1"}))
					Expect(metadata).To(Equal(atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					}))

					space, version, metadata = fakeCheckEventHandler.DiscoveredArgsForCall(1)
					Expect(space).To(Equal(atc.Space("space")))
					Expect(version).To(Equal(atc.Version{"ref": "v2"}))
					Expect(metadata).To(Equal(atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					}))

					space, version, metadata = fakeCheckEventHandler.DiscoveredArgsForCall(2)
					Expect(space).To(Equal(atc.Space("space2")))
					Expect(version).To(Equal(atc.Version{"ref": "v1"}))
					Expect(metadata).To(Equal(atc.Metadata{
						atc.MetadataField{
							Name:  "some",
							Value: "metadata",
						},
					}))

					Expect(fakeCheckEventHandler.LatestVersionsCallCount()).To(Equal(1))
					Expect(checkErr).ToNot(HaveOccurred())
				})
			})

			Context("when running artifact check fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					runCheckError = disaster
				})

				It("returns the error", func() {
					Expect(checkErr).To(Equal(disaster))
				})
			})

			Context("when artifact check exits nonzero", func() {
				BeforeEach(func() {
					checkScriptStderr = "some-stderr"
					checkScriptExitStatus = 9
				})

				It("returns an error containing stderr of the process", func() {
					Expect(checkErr).To(HaveOccurred())

					Expect(checkErr.Error()).To(ContainSubstring("exit status 9"))
					Expect(checkErr.Error()).To(ContainSubstring("some-stderr"))
				})
			})

			Context("when the response of artifact check is malformed", func() {
				BeforeEach(func() {
					tarStream := new(bytes.Buffer)

					tarWriter := tar.NewWriter(tarStream)

					response = []byte(`malformed`)

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

				It("returns an error", func() {
					Expect(checkErr).To(HaveOccurred())
				})
			})

			Context("when the response has an unknown action", func() {
				BeforeEach(func() {
					tarStream := new(bytes.Buffer)

					tarWriter := tar.NewWriter(tarStream)

					response = []byte(`
			{"action": "unknown-action", "space": "some-space", "version": {"ref": "v1"}}`)

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

				It("returns action not found error", func() {
					Expect(checkErr).To(HaveOccurred())
					Expect(checkErr).To(Equal(v2.ActionNotFoundError{Action: "unknown-action"}))
				})
			})

			Context("when streaming out fails", func() {
				BeforeEach(func() {
					fakeContainer.StreamOutReturns(nil, errors.New("ah"))
				})

				It("returns the error", func() {
					Expect(checkErr).To(HaveOccurred())
				})
			})

			Context("when streaming out non tar response", func() {
				BeforeEach(func() {
					streamedOut := gbytes.NewBuffer()
					fakeContainer.StreamOutReturns(streamedOut, nil)
				})

				It("returns an error", func() {
					Expect(checkErr).To(HaveOccurred())
				})
			})
		})
	})

	Context("when a signal is received", func() {
		var waited chan<- struct{}
		var done chan struct{}

		BeforeEach(func() {
			fakeContainer.AttachReturns(nil, errors.New("not-found"))
			fakeContainer.RunReturns(checkScriptProcess, nil)

			waiting := make(chan struct{})
			done = make(chan struct{})
			waited = waiting

			checkScriptProcess.WaitStub = func() (int, error) {
				// cause waiting to block so that it can be aborted
				<-waiting
				return 0, nil
			}

			fakeContainer.StopStub = func(bool) error {
				close(waited)
				return nil
			}

			go func() {
				checkErr = resource.Check(ctx, fakeCheckEventHandler, source, spaceVersion)
				close(done)
			}()
		})

		It("stops the container", func() {
			cancel()
			<-done
			Expect(fakeContainer.StopCallCount()).To(Equal(1))
			Expect(fakeContainer.StopArgsForCall(0)).To(BeFalse())
			Expect(checkErr).To(Equal(context.Canceled))
		})

		It("doesn't send garden terminate signal to process", func() {
			cancel()
			<-done
			Expect(checkErr).To(Equal(context.Canceled))
			Expect(checkScriptProcess.SignalCallCount()).To(BeZero())
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
				Expect(checkErr).To(Equal(context.Canceled))
			})
		})
	})
})

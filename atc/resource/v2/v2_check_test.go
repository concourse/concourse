package v2_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/resource/v2/v2fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource Check", func() {
	var (
		source       atc.Source
		spaceVersion map[atc.Space]atc.Version

		checkScriptStderr     string
		checkScriptExitStatus int
		runCheckError         error

		checkScriptProcess    *gardenfakes.FakeProcess
		fakeCheckEventHandler *v2fakes.FakeCheckEventHandler

		checkErr error
		response []byte
	)

	BeforeEach(func() {
		source = atc.Source{"some": "source"}
		fakeCheckEventHandler = new(v2fakes.FakeCheckEventHandler)
		spaceVersion = map[atc.Space]atc.Version{"space": atc.Version{"some": "version"}}

		checkScriptStderr = ""
		checkScriptExitStatus = 0
		runCheckError = nil

		checkScriptProcess = new(gardenfakes.FakeProcess)
		checkScriptProcess.WaitStub = func() (int, error) {
			return checkScriptExitStatus, nil
		}

		checkErr = nil

		response = []byte(`
		{"action": "default_space", "space": "space"}
		{"action": "discovered", "space": "space", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}
		{"action": "discovered", "space": "space", "version": {"ref": "v2"}, "metadata": [{"name": "some", "value": "metadata"}]}
		{"action": "discovered", "space": "space2", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}`)
	})

	JustBeforeEach(func() {
		fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
			request, err := ioutil.ReadAll(io.Stdin)
			Expect(err).NotTo(HaveOccurred())

			var checkReq v2.CheckRequest
			err = json.Unmarshal(request, &checkReq)
			Expect(err).NotTo(HaveOccurred())

			Expect(checkReq.Config).To(Equal(map[string]interface{}(source)))
			Expect(checkReq.From).To(Equal(spaceVersion))
			Expect(checkReq.ResponsePath).ToNot(BeEmpty())

			err = ioutil.WriteFile(checkReq.ResponsePath, response, 0644)
			Expect(err).NotTo(HaveOccurred())

			if runCheckError != nil {
				return nil, runCheckError
			}

			_, err = io.Stderr.Write([]byte(checkScriptStderr))
			Expect(err).NotTo(HaveOccurred())

			return checkScriptProcess, nil
		}

		checkErr = resource.Check(context.TODO(), fakeCheckEventHandler, source, spaceVersion)
	})

	It("runs check artifact with the request on stdin", func() {
		Expect(checkErr).ToNot(HaveOccurred())

		spec, _ := fakeContainer.RunArgsForCall(0)
		Expect(spec.Path).To(Equal(resourceInfo.Artifacts.Check))
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
			response = []byte(`malformed`)
		})

		It("returns an error", func() {
			Expect(checkErr).To(HaveOccurred())
		})
	})

	Context("when the response has an unknown action", func() {
		BeforeEach(func() {
			response = []byte(`
			{"action": "unknown-action", "space": "some-space", "version": {"ref": "v1"}}`)
		})

		It("returns action not found error", func() {
			Expect(checkErr).To(HaveOccurred())
			Expect(checkErr).To(Equal(v2.ActionNotFoundError{Action: "unknown-action"}))
		})
	})
})

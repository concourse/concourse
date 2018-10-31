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

		checkScriptProcess *gardenfakes.FakeProcess

		checkErr error
		response []byte
	)

	BeforeEach(func() {
		source = atc.Source{"some": "source"}
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
			{"default_space": "space"}
			{"space": "space", "version": {"ref": "v2"}, "metadata": [{"name": "some", "value": "metadata"}]}
			{"space": "space", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}
			{"space": "space2", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}`)
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

		checkErr = resource.Check(context.TODO(), source, spaceVersion)
	})

	It("runs check artifact with the request on stdin", func() {
		Expect(checkErr).NotTo(HaveOccurred())

		spec, _ := fakeContainer.RunArgsForCall(0)
		Expect(spec.Path).To(Equal(resourceInfo.Artifacts.Check))
	})

	Context("when the default space is not null", func() {
		It("saves the default space", func() {
			Expect(fakeResourceConfig.SaveDefaultSpaceCallCount()).To(Equal(1))
			Expect(fakeResourceConfig.SaveDefaultSpaceArgsForCall(0)).To(Equal(atc.Space("space")))
		})
	})

	Context("when the default space is null", func() {
		BeforeEach(func() {
			response = []byte(`
			{"default_space": null}
			{"space": "space", "version": {"ref": "v2"}, "metadata": [{"name": "some", "value": "metadata"}]}
			{"space": "space", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}
			{"space": "space2", "version": {"ref": "v1"}, "metadata": [{"name": "some", "value": "metadata"}]}`)
		})

		It("does not save the default space", func() {
			Expect(fakeResourceConfig.SaveDefaultSpaceCallCount()).To(Equal(0))
		})
	})

	It("saves the versions and all spaces", func() {
		Expect(fakeResourceConfig.SaveVersionCallCount()).To(Equal(3))
		Expect(fakeResourceConfig.SaveVersionArgsForCall(0)).To(Equal(atc.SpaceVersion{
			Space:   atc.Space("space"),
			Version: atc.Version{"ref": "v2"},
			Metadata: atc.Metadata{
				atc.MetadataField{
					Name:  "some",
					Value: "metadata",
				},
			},
		}))
		Expect(fakeResourceConfig.SaveVersionArgsForCall(1)).To(Equal(atc.SpaceVersion{
			Space:   atc.Space("space"),
			Version: atc.Version{"ref": "v1"},
			Metadata: atc.Metadata{
				atc.MetadataField{
					Name:  "some",
					Value: "metadata",
				},
			},
		}))
		Expect(fakeResourceConfig.SaveVersionArgsForCall(2)).To(Equal(atc.SpaceVersion{
			Space:   atc.Space("space2"),
			Version: atc.Version{"ref": "v1"},
			Metadata: atc.Metadata{
				atc.MetadataField{
					Name:  "some",
					Value: "metadata",
				},
			},
		}))

		Expect(fakeResourceConfig.SaveSpaceCallCount()).To(Equal(2))
		Expect(fakeResourceConfig.SaveSpaceArgsForCall(0)).To(Equal(atc.Space("space")))
		Expect(fakeResourceConfig.SaveSpaceArgsForCall(1)).To(Equal(atc.Space("space2")))
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
})

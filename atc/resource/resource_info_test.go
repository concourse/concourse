package resource_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/concourse/atc/resource/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource Info", func() {
	var (
		infoScriptStdout     string
		infoScriptStderr     string
		infoScriptExitStatus int
		runInfoError         error

		infoScriptProcess *gardenfakes.FakeProcess

		infoResult v2.ResourceInfo
		infoErr    error
	)

	BeforeEach(func() {
		infoScriptStdout = "{}"
		infoScriptStderr = ""
		infoScriptExitStatus = 0
		runInfoError = nil

		infoScriptProcess = new(gardenfakes.FakeProcess)
		infoScriptProcess.WaitStub = func() (int, error) {
			return infoScriptExitStatus, nil
		}

		infoResult = v2.ResourceInfo{}
		infoErr = nil
	})

	JustBeforeEach(func() {
		fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
			if runInfoError != nil {
				return nil, runInfoError
			}

			_, err := io.Stdout.Write([]byte(infoScriptStdout))
			Expect(err).NotTo(HaveOccurred())

			_, err = io.Stderr.Write([]byte(infoScriptStderr))
			Expect(err).NotTo(HaveOccurred())

			return infoScriptProcess, nil
		}

		infoResult, infoErr = unversionedResource.Info(context.TODO())
	})

	It("runs /info", func() {
		Expect(infoErr).NotTo(HaveOccurred())

		spec, _ := fakeContainer.RunArgsForCall(0)
		Expect(spec.Path).To(Equal("/info"))
		Expect(spec.Args).To(BeEmpty())
	})

	Context("when /info outputs artifacts", func() {
		BeforeEach(func() {
			infoScriptStdout = `{
				"artifacts": {
					"api_version":"2.0",
					"check":"artifact check",
					"get":"artifact get",
					"put":"artifact put"
				}
			}`
		})

		It("returns the raw parsed contents", func() {
			Expect(infoErr).NotTo(HaveOccurred())

			Expect(infoResult).To(Equal(v2.ResourceInfo{
				Artifacts: v2.Artifacts{
					APIVersion: "2.0",
					Check:      "artifact check",
					Get:        "artifact get",
					Put:        "artifact put",
				},
			}))
		})
	})

	Context("when running /info fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			runInfoError = disaster
		})

		It("returns the error", func() {
			Expect(infoErr).To(Equal(disaster))
		})
	})

	Context("when /info exits nonzero", func() {
		BeforeEach(func() {
			infoScriptStderr = "some-stderr"
			infoScriptExitStatus = 9
		})

		It("returns an error containing stderr of the process", func() {
			Expect(infoErr).To(HaveOccurred())

			Expect(infoErr.Error()).To(ContainSubstring("exit status 9"))
			Expect(infoErr.Error()).To(ContainSubstring("some-stderr"))
		})
	})

	Context("when the output of /info is malformed", func() {
		BeforeEach(func() {
			infoScriptStdout = "ß"
		})

		It("returns an error", func() {
			Expect(infoErr).To(HaveOccurred())
		})
	})
})

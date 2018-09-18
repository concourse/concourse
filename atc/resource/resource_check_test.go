package resource_test

import (
	"context"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource Check", func() {
	var (
		source  atc.Source
		version atc.Version

		checkScriptStdout     string
		checkScriptStderr     string
		checkScriptExitStatus int
		runCheckError         error

		checkScriptProcess *gardenfakes.FakeProcess

		checkResult []atc.Version
		checkErr    error
	)

	BeforeEach(func() {
		source = atc.Source{"some": "source"}
		version = atc.Version{"some": "version"}

		checkScriptStdout = "[]"
		checkScriptStderr = ""
		checkScriptExitStatus = 0
		runCheckError = nil

		checkScriptProcess = new(gardenfakes.FakeProcess)
		checkScriptProcess.WaitStub = func() (int, error) {
			return checkScriptExitStatus, nil
		}

		checkResult = nil
		checkErr = nil
	})

	JustBeforeEach(func() {
		fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
			if runCheckError != nil {
				return nil, runCheckError
			}

			_, err := io.Stdout.Write([]byte(checkScriptStdout))
			Expect(err).NotTo(HaveOccurred())

			_, err = io.Stderr.Write([]byte(checkScriptStderr))
			Expect(err).NotTo(HaveOccurred())

			return checkScriptProcess, nil
		}

		checkResult, checkErr = resourceForContainer.Check(context.TODO(), source, version)
	})

	It("runs /opt/resource/check the request on stdin", func() {
		Expect(checkErr).NotTo(HaveOccurred())

		spec, io := fakeContainer.RunArgsForCall(0)
		Expect(spec.Path).To(Equal("/opt/resource/check"))
		Expect(spec.Args).To(BeEmpty())

		request, err := ioutil.ReadAll(io.Stdin)
		Expect(err).NotTo(HaveOccurred())

		Expect(string(request)).To(Equal(`{"source":{"some":"source"},"version":{"some":"version"}}`))
	})

	Context("when /check outputs versions", func() {
		BeforeEach(func() {
			checkScriptStdout = `[{"ver":"abc"}, {"ver":"def"}, {"ver":"ghi"}]`
		})

		It("returns the raw parsed contents", func() {
			Expect(checkErr).NotTo(HaveOccurred())

			Expect(checkResult).To(Equal([]atc.Version{
				atc.Version{"ver": "abc"},
				atc.Version{"ver": "def"},
				atc.Version{"ver": "ghi"},
			}))

		})
	})

	Context("when running /opt/resource/check fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			runCheckError = disaster
		})

		It("returns the error", func() {
			Expect(checkErr).To(Equal(disaster))
		})
	})

	Context("when /opt/resource/check exits nonzero", func() {
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

	Context("when the output of /opt/resource/check is malformed", func() {
		BeforeEach(func() {
			checkScriptStdout = "ß"
		})

		It("returns an error", func() {
			Expect(checkErr).To(HaveOccurred())
		})
	})
})

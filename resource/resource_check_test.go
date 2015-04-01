package resource_test

import (
	"errors"
	"io/ioutil"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
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

		checkScriptProcess *gfakes.FakeProcess

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

		checkScriptProcess = new(gfakes.FakeProcess)
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
			Ω(err).ShouldNot(HaveOccurred())

			_, err = io.Stderr.Write([]byte(checkScriptStderr))
			Ω(err).ShouldNot(HaveOccurred())

			return checkScriptProcess, nil
		}

		checkResult, checkErr = resource.Check(source, version)
	})

	It("runs /opt/resource/check the request on stdin", func() {
		Ω(checkErr).ShouldNot(HaveOccurred())

		spec, io := fakeContainer.RunArgsForCall(0)
		Ω(spec.Path).Should(Equal("/opt/resource/check"))
		Ω(spec.Args).Should(BeEmpty())
		Ω(spec.Privileged).Should(BeTrue())
		Ω(spec.TTY).Should(Equal(&garden.TTYSpec{}))

		request, err := ioutil.ReadAll(io.Stdin)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(string(request)).Should(Equal(`{"source":{"some":"source"},"version":{"some":"version"}}`))
	})

	Context("when /check outputs versions", func() {
		BeforeEach(func() {
			checkScriptStdout = `[{"ver":"abc"}, {"ver":"def"}, {"ver":"ghi"}]`
		})

		It("returns the raw parsed contents", func() {
			Ω(checkErr).ShouldNot(HaveOccurred())

			Ω(checkResult).Should(Equal([]atc.Version{
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
			Ω(checkErr).Should(Equal(disaster))
		})
	})

	Context("when /opt/resource/check exits nonzero", func() {
		BeforeEach(func() {
			checkScriptStderr = "some-stderr"
			checkScriptExitStatus = 9
		})

		It("returns an error containing stderr of the process", func() {
			Ω(checkErr).Should(HaveOccurred())

			Ω(checkErr.Error()).Should(ContainSubstring("exit status 9"))
			Ω(checkErr.Error()).Should(ContainSubstring("some-stderr"))
		})
	})

	Context("when the output of /opt/resource/check is malformed", func() {
		BeforeEach(func() {
			checkScriptStdout = "ß"
		})

		It("returns an error", func() {
			Ω(checkErr).Should(HaveOccurred())
		})
	})
})

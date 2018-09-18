package exec_test

import (
	"bytes"
	"context"
	"io/ioutil"

	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/exec/execfakes"
	"github.com/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UserArtifactStep", func() {
	var (
		ctx    context.Context
		cancel func()

		state    exec.RunState
		delegate *execfakes.FakeBuildStepDelegate

		step    exec.Step
		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		state = exec.NewRunState()

		delegate = new(execfakes.FakeBuildStepDelegate)
		delegate.StdoutReturns(ioutil.Discard)
	})

	AfterEach(func() {
		cancel()
	})

	JustBeforeEach(func() {
		step = exec.UserArtifact(
			"some-plan-id",
			"some-name",
			delegate,
		)

		stepErr = step.Run(ctx, state)
	})

	It("is successful", func() {
		Expect(stepErr).ToNot(HaveOccurred())
		Expect(step.Succeeded()).To(BeTrue())
	})

	It("registers an artifact which reads from user input", func() {
		source, found := state.Artifacts().SourceFor("some-name")
		Expect(found).To(BeTrue())

		dest := new(workerfakes.FakeArtifactDestination)

		input := ioutil.NopCloser(bytes.NewBufferString("hello"))
		go state.SendUserInput("some-plan-id", input)

		Expect(dest.StreamInCallCount()).To(Equal(0))
		Expect(source.StreamTo(dest)).To(Succeed())
		Expect(dest.StreamInCallCount()).To(Equal(1))

		path, stream := dest.StreamInArgsForCall(0)
		Expect(path).To(Equal("."))
		Expect(ioutil.ReadAll(stream)).To(Equal([]byte("hello")))
	})
})

package fly_test

import (
	"fmt"
	"os/exec"

	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func shouldRunSuccessfully(passedArgs ...string) {
	args := append([]string{"-t", targetedConcourse}, passedArgs...)
	fly := exec.Command(flyBin, args...)

	session := helpers.StartFly(fly)
	<-session.Exited
	Expect(session.ExitCode()).To(Equal(0))
}

func pipelineName() string { return fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode()) }

var _ = Describe("the quality of being authenticated", func() {
	DescribeTable("running commands with pipeline name when authenticated",
		func(command string) {
			shouldRunSuccessfully("set-pipeline", "-p", pipelineName(), "-c", "../fixtures/simple-pipeline.yml", "-n")
			shouldRunSuccessfully(command, "-p", pipelineName())
		},
		Entry("get-pipeline", "get-pipeline"),
		Entry("pause-pipeline", "pause-pipeline"),
		Entry("unpause-pipeline", "unpause-pipeline"),
		Entry("checklist", "checklist"),
	)

	DescribeTable("running commands when authenticated",
		func(args ...string) {
			shouldRunSuccessfully("set-pipeline", "-p", pipelineName(), "-c", "../fixtures/simple-pipeline.yml", "-n")
			shouldRunSuccessfully(args...)
		},
		Entry("containers", "containers"),
		Entry("volumes", "volumes"),
		Entry("workers", "workers"),
		Entry("execute", "execute", "-c", "../fixtures/simple-task.yml"),
		Entry("watch", "watch"),
	)

	DescribeTable("running commands that require confirmation when authenticated",
		func(args ...string) {
			shouldRunSuccessfully("set-pipeline", "-p", pipelineName(), "-c", "../fixtures/simple-pipeline.yml", "-n")
			shouldRunSuccessfully(append(args, "-p", pipelineName(), "-n")...)
		},
		Entry("destroy-pipeline", "destroy-pipeline"),
		Entry("set-pipeline", "set-pipeline", "-c", "../fixtures/simple-pipeline.yml"),
	)

	It("can hijack successfully", func() {
		fly := exec.Command(flyBin, "-t", targetedConcourse, "hijack", "/bin/sh")

		stdin, err := fly.StdinPipe()
		Expect(err).ToNot(HaveOccurred())

		defer stdin.Close()

		session := helpers.StartFly(fly)
		fmt.Fprint(stdin, "1\n")

		fmt.Fprint(stdin, "exit\n")

		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
	})
})

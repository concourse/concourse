package fly_test

import (
	"fmt"
	"os/exec"

	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func shouldRunSuccesfully(passedArgs ...string) {
	args := append([]string{"-t", targetedConcourse}, passedArgs...)
	fly := exec.Command(flyBin, args...)
	session := helpers.StartFly(fly)

	Eventually(session).Should(gexec.Exit(0))
}

func shouldRunSuccesfullyWithConfirmation(passedArgs ...string) {
	args := append([]string{"-t", targetedConcourse}, passedArgs...)
	fly := exec.Command(flyBin, args...)

	stdin, err := fly.StdinPipe()
	Expect(err).ToNot(HaveOccurred())

	defer stdin.Close()

	session := helpers.StartFly(fly)

	Eventually(session.Out).Should(gbytes.Say("yN"))
	fmt.Fprint(stdin, "y\n")

	Eventually(session).Should(gexec.Exit(0))
}

func pipelineName() string { return fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode()) }

var _ = Describe("the quality of being authenticated", func() {
	DescribeTable("running commands with pipeline name when authenticated",
		func(command string) {
			shouldRunSuccesfullyWithConfirmation("set-pipeline", "-p", pipelineName(), "-c", "../fixtures/simple-pipeline.yml")
			shouldRunSuccesfully(command, "-p", pipelineName())
		},
		Entry("get-pipeline", "get-pipeline"),
		Entry("pause-pipeline", "pause-pipeline"),
		Entry("unpause-pipeline", "unpause-pipeline"),
		Entry("checklist", "checklist"),
	)

	DescribeTable("running commands when authenticated",
		func(args ...string) {
			shouldRunSuccesfullyWithConfirmation("set-pipeline", "-p", pipelineName(), "-c", "../fixtures/simple-pipeline.yml")
			shouldRunSuccesfully(args...)
		},
		Entry("containers", "containers"),
		Entry("volumes", "volumes"),
		Entry("workers", "workers"),
		Entry("execute", "execute", "-c", "../fixtures/simple-task.yml"),
		Entry("watch", "watch"),
	)

	DescribeTable("running commands that require confirmation when authenticated",
		func(args ...string) {
			shouldRunSuccesfullyWithConfirmation("set-pipeline", "-p", pipelineName(), "-c", "../fixtures/simple-pipeline.yml")
			shouldRunSuccesfullyWithConfirmation(append(args, "-p", pipelineName())...)
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

		fmt.Fprint(stdin, "exit\n")

		Eventually(session).Should(gexec.Exit(0))
	})
})

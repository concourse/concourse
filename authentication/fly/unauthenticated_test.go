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

var _ = Describe("the quality of being unauthenticated", func() {
	BeforeEach(func() {
		noAuth, _, _, err := helpers.GetAuthMethods(atcURL)
		Expect(err).ToNot(HaveOccurred())
		if noAuth {
			Skip("No auth methods enabled; skipping unauthenticated tests")
		}
	})

	DescribeTable("trying to run commands when unauthenticated",
		func(passedArgs ...string) {
			args := append([]string{"-t", atcURL}, passedArgs...)
			fly := exec.Command(flyBin, args...)
			session := helpers.StartFly(fly)

			Eventually(session).Should(gexec.Exit(1))

			Expect(session.Err).Should(gbytes.Say("401 Unauthorized"))
		},

		Entry("get-pipeline", "get-pipeline", "-p", "john"),
		Entry("set-pipeline", "set-pipeline", "-p", "john", "-c", "../fixtures/simple-pipeline.yml"),
		Entry("pause-pipeline", "pause-pipeline", "-p", "john"),
		Entry("unpause-pipeline", "unpause-pipeline", "-p", "john"),
		Entry("checklist", "checklist", "-p", "john"),
		Entry("containers", "containers"),
		Entry("volumes", "volumes"),
		Entry("workers", "workers"),
		Entry("execute", "execute", "-c", "../fixtures/simple-task.yml"),
		Entry("watch", "watch"),
		Entry("hijack", "hijack"),
	)

	DescribeTable("trying to run commands that require confirmation when unauthenticated",
		func(passedArgs ...string) {
			args := append([]string{"-t", atcURL}, passedArgs...)
			fly := exec.Command(flyBin, args...)

			stdin, err := fly.StdinPipe()
			Expect(err).ToNot(HaveOccurred())

			session := helpers.StartFly(fly)

			Eventually(session.Out).Should(gbytes.Say("are you sure?"))
			fmt.Fprint(stdin, "y\n")

			Eventually(session).Should(gexec.Exit(1))

			Expect(session.Err).Should(gbytes.Say("Status: 401 Unauthorized"))
		},

		Entry("destroy-pipeline", "destroy-pipeline", "-p", "john"),
	)
})
